package bus

import (
	"context"
	"sync"
)

// MessageBus is a hub-and-spoke message bus using Go channels.
type MessageBus struct {
	inbound  chan InboundMessage
	outbound chan OutboundMessage
	subs     map[string][]func(OutboundMessage) // channel name -> subscribers
	mu       sync.RWMutex
	bufSize  int
}

// NewMessageBus creates a new MessageBus with the given buffer size.
// If bufSize is 0, defaults to 100.
func NewMessageBus(bufSize int) *MessageBus {
	if bufSize <= 0 {
		bufSize = 100
	}
	return &MessageBus{
		inbound:  make(chan InboundMessage, bufSize),
		outbound: make(chan OutboundMessage, bufSize),
		subs:     make(map[string][]func(OutboundMessage)),
		bufSize:  bufSize,
	}
}

// PublishInbound sends an inbound message onto the bus.
func (b *MessageBus) PublishInbound(msg InboundMessage) {
	b.inbound <- msg
}

// PublishOutbound sends an outbound message onto the bus.
func (b *MessageBus) PublishOutbound(msg OutboundMessage) {
	b.outbound <- msg
}

// ConsumeInbound blocks until an inbound message is available or ctx is cancelled.
func (b *MessageBus) ConsumeInbound(ctx context.Context) (InboundMessage, error) {
	select {
	case msg, ok := <-b.inbound:
		if !ok {
			return InboundMessage{}, context.Canceled
		}
		return msg, nil
	case <-ctx.Done():
		return InboundMessage{}, ctx.Err()
	}
}

// Subscribe registers fn to receive outbound messages for the given channel.
// An empty channel string subscribes to ALL channels.
func (b *MessageBus) Subscribe(channel string, fn func(OutboundMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[channel] = append(b.subs[channel], fn)
}

// DispatchOutbound runs in a goroutine, reading outbound messages and
// delivering them to matching subscribers. Returns when ctx is cancelled
// or the outbound channel is closed.
func (b *MessageBus) DispatchOutbound(ctx context.Context) {
	for {
		select {
		case msg, ok := <-b.outbound:
			if !ok {
				return
			}
			b.dispatch(msg)
		case <-ctx.Done():
			return
		}
	}
}

// dispatch delivers msg to all matching subscribers (channel-specific + wildcard).
func (b *MessageBus) dispatch(msg OutboundMessage) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// channel-specific subscribers
	for _, fn := range b.subs[msg.Channel] {
		fn(msg)
	}
	// wildcard subscribers (empty string = all channels)
	for _, fn := range b.subs[""] {
		fn(msg)
	}
}

// Close closes both the inbound and outbound channels.
func (b *MessageBus) Close() {
	close(b.inbound)
	close(b.outbound)
}
