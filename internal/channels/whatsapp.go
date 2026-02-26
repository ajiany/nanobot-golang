package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/coopco/nanobot/internal/bus"
)

func init() {
	Register("whatsapp", newWhatsAppChannel)
}

type whatsAppConfig struct {
	AccessToken   string   `json:"access_token"`
	PhoneNumberID string   `json:"phone_number_id"`
	VerifyToken   string   `json:"verify_token"`
	WebhookPort   int      `json:"webhook_port"`
	AllowedUsers  []string `json:"allowed_users"`
}

// WhatsAppChannel implements Channel for WhatsApp via the Cloud API (HTTP webhooks).
type WhatsAppChannel struct {
	accessToken   string
	phoneNumberID string
	verifyToken   string
	bus           *bus.MessageBus
	allowedUsers  map[string]bool
	server        *http.Server
}

func newWhatsAppChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c whatsAppConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.WebhookPort == 0 {
		c.WebhookPort = 9005
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &WhatsAppChannel{
		accessToken:   c.AccessToken,
		phoneNumberID: c.PhoneNumberID,
		verifyToken:   c.VerifyToken,
		bus:           msgBus,
		allowedUsers:  allowed,
		server:        &http.Server{Addr: fmt.Sprintf(":%d", c.WebhookPort)},
	}, nil
}

func (c *WhatsAppChannel) Name() string { return "whatsapp" }

func (c *WhatsAppChannel) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", c.handleWebhook)
	c.server.Handler = mux

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("whatsapp: server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

func (c *WhatsAppChannel) Stop() error {
	return c.server.Shutdown(context.Background())
}

func (c *WhatsAppChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// GET: webhook verification
	if r.Method == http.MethodGet {
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")
		if mode == "subscribe" && token == c.verifyToken {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, challenge)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		return
	}

	// POST: incoming message
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload struct {
		Entry []struct {
			Changes []struct {
				Value struct {
					Messages []struct {
						From string `json:"from"`
						ID   string `json:"id"`
						Text struct {
							Body string `json:"body"`
						} `json:"text"`
						Type string `json:"type"`
					} `json:"messages"`
				} `json:"value"`
			} `json:"changes"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" {
					continue
				}
				senderID := msg.From
				if !c.IsAllowed(senderID) {
					slog.Warn("whatsapp: message from disallowed user", "user", senderID)
					continue
				}
				c.bus.PublishInbound(bus.InboundMessage{
					Channel:  "whatsapp",
					SenderID: senderID,
					ChatID:   senderID,
					Content:  msg.Text.Body,
				})
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (c *WhatsAppChannel) Send(msg bus.OutboundMessage) error {
	body, _ := json.Marshal(map[string]any{
		"messaging_product": "whatsapp",
		"to":                msg.ChatID,
		"type":              "text",
		"text":              map[string]string{"body": msg.Content},
	})
	url := fmt.Sprintf("https://graph.facebook.com/v21.0/%s/messages", c.phoneNumberID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp: send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp: send message status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *WhatsAppChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
