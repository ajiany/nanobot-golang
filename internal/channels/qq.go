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
	Register("qq", newQQChannel)
}

type qqConfig struct {
	AppID        string   `json:"appId"`
	Token        string   `json:"token"`
	AppSecret    string   `json:"appSecret"`
	WebhookPort  int      `json:"webhookPort"`
	AllowedUsers []string `json:"allowedUsers"`
}

// QQChannel implements Channel for QQ Official Bot via HTTP webhook.
type QQChannel struct {
	appID        string
	token        string
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	server       *http.Server
}

func newQQChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c qqConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.WebhookPort == 0 {
		c.WebhookPort = 9003
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &QQChannel{
		appID:        c.AppID,
		token:        c.Token,
		bus:          msgBus,
		allowedUsers: allowed,
		server:       &http.Server{Addr: fmt.Sprintf(":%d", c.WebhookPort)},
	}, nil
}

func (c *QQChannel) Name() string { return "qq" }

func (c *QQChannel) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleEvent)
	c.server.Handler = mux

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("qq: server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

func (c *QQChannel) handleEvent(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// URL verification challenge
	var challenge struct {
		Op int `json:"op"`
		D  struct {
			PlainToken string `json:"plain_token"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &challenge); err == nil && challenge.Op == 13 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"plain_token": challenge.D.PlainToken})
		return
	}

	// Message event (op=0, t=AT_MESSAGE_CREATE or DIRECT_MESSAGE_CREATE)
	var event struct {
		Op int    `json:"op"`
		T  string `json:"t"`
		D  struct {
			ID        string `json:"id"`
			ChannelID string `json:"channel_id"`
			Author    struct {
				ID string `json:"id"`
			} `json:"author"`
			Content string `json:"content"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}

	if event.Op != 0 || (event.T != "AT_MESSAGE_CREATE" && event.T != "DIRECT_MESSAGE_CREATE") {
		w.WriteHeader(http.StatusOK)
		return
	}

	senderID := event.D.Author.ID
	if !c.IsAllowed(senderID) {
		slog.Warn("qq: message from disallowed user", "user", senderID)
		w.WriteHeader(http.StatusOK)
		return
	}

	c.bus.PublishInbound(bus.InboundMessage{
		Channel:  "qq",
		SenderID: senderID,
		ChatID:   event.D.ChannelID,
		Content:  event.D.Content,
	})
	w.WriteHeader(http.StatusOK)
}

func (c *QQChannel) Stop() error {
	return c.server.Shutdown(context.Background())
}

func (c *QQChannel) Send(msg bus.OutboundMessage) error {
	body, _ := json.Marshal(map[string]string{
		"content": msg.Content,
	})
	url := fmt.Sprintf("https://api.sgroup.qq.com/channels/%s/messages", msg.ChatID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bot %s.%s", c.appID, c.token))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("qq: send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("qq: send message status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *QQChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
