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
	Register("feishu", newFeishuChannel)
}

type feishuConfig struct {
	AppID        string   `json:"appId"`
	AppSecret    string   `json:"appSecret"`
	WebhookPort  int      `json:"webhookPort"`
	AllowedUsers []string `json:"allowedUsers"`
}

// FeishuChannel implements Channel for Feishu (Lark) via HTTP webhooks.
type FeishuChannel struct {
	appID        string
	appSecret    string
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	server       *http.Server
	accessToken  string
}

func newFeishuChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c feishuConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.WebhookPort == 0 {
		c.WebhookPort = 9001
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &FeishuChannel{
		appID:        c.AppID,
		appSecret:    c.AppSecret,
		bus:          msgBus,
		allowedUsers: allowed,
		server:       &http.Server{Addr: fmt.Sprintf(":%d", c.WebhookPort)},
	}, nil
}

func (c *FeishuChannel) Name() string { return "feishu" }

func (c *FeishuChannel) Start(ctx context.Context) error {
	if err := c.refreshToken(); err != nil {
		return fmt.Errorf("feishu: get access token: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleEvent)
	c.server.Handler = mux

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("feishu: server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

func (c *FeishuChannel) refreshToken() error {
	body, _ := json.Marshal(map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	})
	resp, err := http.Post(
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result struct {
		TenantAccessToken string `json:"tenant_access_token"`
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu auth error %d: %s", result.Code, result.Msg)
	}
	c.accessToken = result.TenantAccessToken
	return nil
}

func (c *FeishuChannel) handleEvent(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// URL verification challenge
	var challenge struct {
		Challenge string `json:"challenge"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(data, &challenge); err == nil && challenge.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": challenge.Challenge})
		return
	}

	// Message event
	var event struct {
		Header struct {
			EventType string `json:"event_type"`
		} `json:"header"`
		Event struct {
			Sender struct {
				SenderID struct {
					OpenID string `json:"open_id"`
				} `json:"sender_id"`
			} `json:"sender"`
			Message struct {
				ChatID  string `json:"chat_id"`
				Content string `json:"content"`
			} `json:"message"`
		} `json:"event"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}
	if event.Header.EventType != "im.message.receive_v1" {
		w.WriteHeader(http.StatusOK)
		return
	}

	senderID := event.Event.Sender.SenderID.OpenID
	if !c.IsAllowed(senderID) {
		slog.Warn("feishu: message from disallowed user", "user", senderID)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Content is JSON: {"text": "..."}
	var msgContent struct {
		Text string `json:"text"`
	}
	json.Unmarshal([]byte(event.Event.Message.Content), &msgContent)

	c.bus.PublishInbound(bus.InboundMessage{
		Channel:  "feishu",
		SenderID: senderID,
		ChatID:   event.Event.Message.ChatID,
		Content:  msgContent.Text,
	})
	w.WriteHeader(http.StatusOK)
}

func (c *FeishuChannel) Stop() error {
	return c.server.Shutdown(context.Background())
}

func (c *FeishuChannel) Send(msg bus.OutboundMessage) error {
	contentJSON, _ := json.Marshal(map[string]string{"text": msg.Content})
	body, _ := json.Marshal(map[string]string{
		"receive_id": msg.ChatID,
		"msg_type":   "text",
		"content":    string(contentJSON),
	})
	req, err := http.NewRequest(http.MethodPost,
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu: send message status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *FeishuChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
