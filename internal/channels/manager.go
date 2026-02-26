package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/coopco/nanobot/internal/bus"
)

type Manager struct {
	channels []Channel
	bus      *bus.MessageBus
	mu       sync.Mutex
}

func NewManager(msgBus *bus.MessageBus) *Manager {
	m := &Manager{bus: msgBus}
	m.setupOutboundDispatch()
	return m
}

// AddChannel creates and adds a channel from config.
func (m *Manager) AddChannel(name string, cfgJSON json.RawMessage) error {
	factory, ok := GetFactory(name)
	if !ok {
		return fmt.Errorf("no factory registered for channel %q", name)
	}
	ch, err := factory(cfgJSON, m.bus)
	if err != nil {
		return fmt.Errorf("failed to create channel %q: %w", name, err)
	}
	m.mu.Lock()
	m.channels = append(m.channels, ch)
	m.mu.Unlock()
	return nil
}

// StartAll starts all registered channels.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	chs := make([]Channel, len(m.channels))
	copy(chs, m.channels)
	m.mu.Unlock()

	for _, ch := range chs {
		if err := ch.Start(ctx); err != nil {
			return fmt.Errorf("failed to start channel %q: %w", ch.Name(), err)
		}
	}
	return nil
}

// StopAll stops all channels.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	chs := make([]Channel, len(m.channels))
	copy(chs, m.channels)
	m.mu.Unlock()

	var firstErr error
	for _, ch := range chs {
		if err := ch.Stop(); err != nil {
			slog.Error("failed to stop channel", "channel", ch.Name(), "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// setupOutboundDispatch subscribes to outbound messages and routes to channels.
func (m *Manager) setupOutboundDispatch() {
	m.bus.Subscribe("", func(msg bus.OutboundMessage) {
		if msg.Type == "progress" || msg.Type == "tool_hint" {
			return
		}
		m.mu.Lock()
		chs := make([]Channel, len(m.channels))
		copy(chs, m.channels)
		m.mu.Unlock()

		for _, ch := range chs {
			if ch.Name() == msg.Channel {
				if err := ch.Send(msg); err != nil {
					slog.Error("failed to send message", "channel", ch.Name(), "error", err)
				}
				return
			}
		}
	})
}
