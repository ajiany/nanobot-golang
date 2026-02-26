package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func init() {
	Register("mochat", newMochatChannel)
}

type mochatConfig struct {
	URL          string   `json:"url"`
	AllowedUsers []string `json:"allowedUsers"`
}

// MochatChannel implements Channel for Mochat via HTTP long-polling.
type MochatChannel struct {
	baseURL      string
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	cancel       context.CancelFunc
	lastSince    int64
}

func newMochatChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c mochatConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	c.URL = strings.TrimRight(c.URL, "/")
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	return &MochatChannel{
		baseURL:      c.URL,
		bus:          msgBus,
		allowedUsers: allowed,
		lastSince:    time.Now().Unix(),
	}, nil
}

func (c *MochatChannel) Name() string { return "mochat" }

func (c *MochatChannel) Start(ctx context.Context) error {
	pollCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				c.poll()
			}
		}
	}()

	return nil
}

func (c *MochatChannel) poll() {
	url := fmt.Sprintf("%s/api/messages?since=%d", c.baseURL, c.lastSince)
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("mochat: poll error", "err", err)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("mochat: read poll response", "err", err)
		return
	}

	var messages []struct {
		ID        int64  `json:"id"`
		Timestamp int64  `json:"timestamp"`
		SenderID  string `json:"senderId"`
		ChatID    string `json:"chatId"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(data, &messages); err != nil {
		return
	}

	for _, msg := range messages {
		if msg.Timestamp > c.lastSince {
			c.lastSince = msg.Timestamp
		}
		if !c.IsAllowed(msg.SenderID) {
			slog.Warn("mochat: message from disallowed user", "user", msg.SenderID)
			continue
		}
		c.bus.PublishInbound(bus.InboundMessage{
			Channel:  "mochat",
			SenderID: msg.SenderID,
			ChatID:   msg.ChatID,
			Content:  msg.Content,
		})
	}
}

func (c *MochatChannel) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *MochatChannel) Send(msg bus.OutboundMessage) error {
	body, _ := json.Marshal(map[string]string{
		"chatId":  msg.ChatID,
		"content": msg.Content,
	})
	resp, err := http.Post(c.baseURL+"/api/messages", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mochat: send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mochat: send status %d: %s", resp.StatusCode, b)
	}
	return nil
}

func (c *MochatChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
