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
	Register("dingtalk", newDingTalkChannel)
}

type dingtalkConfig struct {
	ClientID     string   `json:"clientId"`
	ClientSecret string   `json:"clientSecret"`
	WebhookPort  int      `json:"webhookPort"`
	AllowedUsers []string `json:"allowedUsers"`
}

// DingTalkChannel implements Channel for DingTalk via HTTP webhooks.
type DingTalkChannel struct {
	clientID     string
	clientSecret string
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	server       *http.Server
	accessToken  string
}

func newDingTalkChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c dingtalkConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.WebhookPort == 0 {
		c.WebhookPort = 9002
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &DingTalkChannel{
		clientID:     c.ClientID,
		clientSecret: c.ClientSecret,
		bus:          msgBus,
		allowedUsers: allowed,
		server:       &http.Server{Addr: fmt.Sprintf(":%d", c.WebhookPort)},
	}, nil
}

func (c *DingTalkChannel) Name() string { return "dingtalk" }

func (c *DingTalkChannel) Start(ctx context.Context) error {
	if err := c.refreshToken(); err != nil {
		return fmt.Errorf("dingtalk: get access token: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.handleEvent)
	c.server.Handler = mux

	go func() {
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("dingtalk: server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

func (c *DingTalkChannel) refreshToken() error {
	body, _ := json.Marshal(map[string]string{
		"clientId":     c.clientID,
		"clientSecret": c.clientSecret,
	})
	resp, err := http.Post(
		"https://api.dingtalk.com/v1.0/oauth2/accessToken",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"accessToken"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("dingtalk auth error %d: %s", result.ErrCode, result.ErrMsg)
	}
	c.accessToken = result.AccessToken
	return nil
}

func (c *DingTalkChannel) handleEvent(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var event struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
		SenderID   string `json:"senderId"`
		ConversationID string `json:"conversationId"`
	}
	if err := json.Unmarshal(data, &event); err != nil {
		http.Error(w, "parse error", http.StatusBadRequest)
		return
	}

	if !c.IsAllowed(event.SenderID) {
		slog.Warn("dingtalk: message from disallowed user", "user", event.SenderID)
		w.WriteHeader(http.StatusOK)
		return
	}

	c.bus.PublishInbound(bus.InboundMessage{
		Channel:  "dingtalk",
		SenderID: event.SenderID,
		ChatID:   event.ConversationID,
		Content:  event.Text.Content,
	})
	w.WriteHeader(http.StatusOK)
}

func (c *DingTalkChannel) Stop() error {
	return c.server.Shutdown(context.Background())
}

func (c *DingTalkChannel) Send(msg bus.OutboundMessage) error {
	msgParam, _ := json.Marshal(map[string]string{"content": msg.Content})
	body, _ := json.Marshal(map[string]interface{}{
		"robotCode": c.clientID,
		"userIds":   []string{msg.ChatID},
		"msgKey":    "sampleText",
		"msgParam":  string(msgParam),
	})
	req, err := http.NewRequest(http.MethodPost,
		"https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", c.accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dingtalk: send message status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *DingTalkChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
