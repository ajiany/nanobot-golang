package channels

import (
	"context"
	"encoding/json"

	"github.com/coopco/nanobot/internal/bus"
)

// Channel is the interface all chat platform channels must implement.
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	Send(msg bus.OutboundMessage) error
	IsAllowed(senderID string) bool
}

// ChannelFactory creates a Channel from JSON config and a MessageBus.
type ChannelFactory func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error)

var registry = map[string]ChannelFactory{}

// Register adds a channel factory to the registry.
func Register(name string, factory ChannelFactory) {
	registry[name] = factory
}

// GetFactory returns the factory for a channel name.
func GetFactory(name string) (ChannelFactory, bool) {
	f, ok := registry[name]
	return f, ok
}

// RegisteredNames returns all registered channel names.
func RegisteredNames() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
