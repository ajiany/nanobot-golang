package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/bwmarrin/discordgo"

	"github.com/coopco/nanobot/internal/bus"
)

func init() {
	Register("discord", newDiscordChannel)
}

type discordConfig struct {
	Token        string   `json:"token"`
	AllowedUsers []string `json:"allowedUsers"`
}

type DiscordChannel struct {
	session      *discordgo.Session
	bus          *bus.MessageBus
	allowedUsers map[string]bool
}

func newDiscordChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var dcfg discordConfig
	if err := json.Unmarshal(cfg, &dcfg); err != nil {
		return nil, fmt.Errorf("failed to parse discord config: %w", err)
	}
	session, err := discordgo.New("Bot " + dcfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %w", err)
	}
	allowed := make(map[string]bool, len(dcfg.AllowedUsers))
	for _, u := range dcfg.AllowedUsers {
		allowed[u] = true
	}
	return &DiscordChannel{
		session:      session,
		bus:          msgBus,
		allowedUsers: allowed,
	}, nil
}

func (c *DiscordChannel) Name() string { return "discord" }

func (c *DiscordChannel) Start(ctx context.Context) error {
	c.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author == nil || m.Author.Bot {
			return
		}
		if !c.IsAllowed(m.Author.ID) {
			slog.Warn("discord: message from disallowed user", "userID", m.Author.ID)
			return
		}
		c.bus.PublishInbound(bus.InboundMessage{
			Channel:  "discord",
			SenderID: m.Author.ID,
			ChatID:   m.ChannelID,
			Content:  m.Content,
		})
	})
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("discord: failed to open websocket: %w", err)
	}
	return nil
}

func (c *DiscordChannel) Stop() error {
	return c.session.Close()
}

func (c *DiscordChannel) Send(msg bus.OutboundMessage) error {
	_, err := c.session.ChannelMessageSend(msg.ChatID, msg.Content)
	if err != nil {
		return fmt.Errorf("discord: failed to send message: %w", err)
	}
	return nil
}

func (c *DiscordChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
