package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func init() {
	Register("slack", newSlackChannel)
}

type slackConfig struct {
	BotToken     string   `json:"botToken"`
	AppToken     string   `json:"appToken"`
	AllowedUsers []string `json:"allowedUsers"`
}

// SlackChannel implements Channel for Slack via socket mode.
type SlackChannel struct {
	client       *slack.Client
	socketClient *socketmode.Client
	bus          *bus.MessageBus
	allowedUsers map[string]bool
}

func newSlackChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var c slackConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(c.AllowedUsers))
	for _, u := range c.AllowedUsers {
		allowed[u] = true
	}
	client := slack.New(c.BotToken, slack.OptionAppLevelToken(c.AppToken))
	socketClient := socketmode.New(client)
	return &SlackChannel{
		client:       client,
		socketClient: socketClient,
		bus:          msgBus,
		allowedUsers: allowed,
	}, nil
}

func (c *SlackChannel) Name() string { return "slack" }

func (c *SlackChannel) Start(ctx context.Context) error {
	go func() {
		for evt := range c.socketClient.Events {
			if evt.Type != socketmode.EventTypeEventsAPI {
				c.socketClient.Ack(*evt.Request)
				continue
			}
			eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				c.socketClient.Ack(*evt.Request)
				continue
			}
			c.socketClient.Ack(*evt.Request)
			if eventsAPI.Type != slackevents.CallbackEvent {
				continue
			}
			inner, ok := eventsAPI.InnerEvent.Data.(*slackevents.MessageEvent)
			if !ok {
				continue
			}
			// skip bot messages
			if inner.BotID != "" {
				continue
			}
			if !c.IsAllowed(inner.User) {
				slog.Warn("slack: message from disallowed user", "user", inner.User)
				continue
			}
			c.bus.PublishInbound(bus.InboundMessage{
				Channel:  "slack",
				SenderID: inner.User,
				ChatID:   inner.Channel,
				Content:  inner.Text,
			})
		}
	}()
	return c.socketClient.RunContext(ctx)
}

func (c *SlackChannel) Stop() error { return nil }

func (c *SlackChannel) Send(msg bus.OutboundMessage) error {
	_, _, err := c.client.PostMessage(msg.ChatID, slack.MsgOptionText(msg.Content, false))
	if err != nil {
		return fmt.Errorf("slack: post message: %w", err)
	}
	return nil
}

func (c *SlackChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
