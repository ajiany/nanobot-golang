package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/coopco/nanobot/internal/bus"
)

func init() {
	Register("telegram", newTelegramChannel)
}

type telegramConfig struct {
	Token        string   `json:"token"`
	AllowedUsers []string `json:"allowedUsers"`
}

type TelegramChannel struct {
	bot          *tgbotapi.BotAPI
	bus          *bus.MessageBus
	allowedUsers map[string]bool
	stopCh       chan struct{}
}

func newTelegramChannel(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
	var tcfg telegramConfig
	if err := json.Unmarshal(cfg, &tcfg); err != nil {
		return nil, fmt.Errorf("failed to parse telegram config: %w", err)
	}
	bot, err := tgbotapi.NewBotAPI(tcfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}
	allowed := make(map[string]bool, len(tcfg.AllowedUsers))
	for _, u := range tcfg.AllowedUsers {
		allowed[u] = true
	}
	return &TelegramChannel{
		bot:          bot,
		bus:          msgBus,
		allowedUsers: allowed,
		stopCh:       make(chan struct{}),
	}, nil
}

func (c *TelegramChannel) Name() string { return "telegram" }

func (c *TelegramChannel) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := c.bot.GetUpdatesChan(u)

	go func() {
		for {
			select {
			case update, ok := <-updates:
				if !ok {
					return
				}
				if update.Message == nil {
					continue
				}
				senderID := strconv.FormatInt(update.Message.From.ID, 10)
				if !c.IsAllowed(senderID) {
					slog.Warn("telegram: message from disallowed user", "senderID", senderID)
					continue
				}
				chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
				c.bus.PublishInbound(bus.InboundMessage{
					Channel:  "telegram",
					SenderID: senderID,
					ChatID:   chatID,
					Content:  update.Message.Text,
				})
			case <-ctx.Done():
				c.bot.StopReceivingUpdates()
				return
			case <-c.stopCh:
				c.bot.StopReceivingUpdates()
				return
			}
		}
	}()
	return nil
}

func (c *TelegramChannel) Stop() error {
	close(c.stopCh)
	return nil
}

func (c *TelegramChannel) Send(msg bus.OutboundMessage) error {
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chatID %q: %w", msg.ChatID, err)
	}
	m := tgbotapi.NewMessage(chatID, msg.Content)
	_, err = c.bot.Send(m)
	return err
}

func (c *TelegramChannel) IsAllowed(senderID string) bool {
	if len(c.allowedUsers) == 0 {
		return true
	}
	return c.allowedUsers[senderID]
}
