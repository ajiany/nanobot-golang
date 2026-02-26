package bus

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPublishConsumeInbound(t *testing.T) {
	tests := []struct {
		name string
		msg  InboundMessage
	}{
		{
			name: "basic message",
			msg:  InboundMessage{Channel: "telegram", SenderID: "u1", ChatID: "c1", Content: "hello"},
		},
		{
			name: "message with metadata",
			msg:  InboundMessage{Channel: "discord", SenderID: "u2", ChatID: "c2", Content: "world", Metadata: map[string]string{"k": "v"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewMessageBus(10)
			b.PublishInbound(tc.msg)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			got, err := b.ConsumeInbound(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Channel != tc.msg.Channel || got.Content != tc.msg.Content {
				t.Errorf("got %+v, want %+v", got, tc.msg)
			}
		})
	}
}

func TestOutboundDispatch(t *testing.T) {
	tests := []struct {
		name    string
		subChan string
		pubChan string
		wantHit bool
	}{
		{"matching channel", "telegram", "telegram", true},
		{"non-matching channel", "discord", "telegram", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := NewMessageBus(10)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var mu sync.Mutex
			var received []OutboundMessage

			b.Subscribe(tc.subChan, func(msg OutboundMessage) {
				mu.Lock()
				received = append(received, msg)
				mu.Unlock()
			})

			go b.DispatchOutbound(ctx)

			b.PublishOutbound(OutboundMessage{Channel: tc.pubChan, ChatID: "c1", Content: "hi", Type: "text"})

			// wait briefly for dispatch
			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			got := len(received) > 0
			mu.Unlock()

			if got != tc.wantHit {
				t.Errorf("received=%v, wantHit=%v", got, tc.wantHit)
			}
		})
	}
}

func TestConsumeInboundCancellation(t *testing.T) {
	b := NewMessageBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := b.ConsumeInbound(ctx)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}

func TestSubscribeAll(t *testing.T) {
	b := NewMessageBus(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var received []OutboundMessage

	// empty string = subscribe to all channels
	b.Subscribe("", func(msg OutboundMessage) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	go b.DispatchOutbound(ctx)

	channels := []string{"telegram", "discord", "system"}
	for _, ch := range channels {
		b.PublishOutbound(OutboundMessage{Channel: ch, Content: "msg"})
	}

	// wait for dispatch
	deadline := time.After(time.Second)
	for {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n >= len(channels) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout: got %d messages, want %d", n, len(channels))
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != len(channels) {
		t.Errorf("got %d messages, want %d", len(received), len(channels))
	}
}

func TestSessionKey(t *testing.T) {
	tests := []struct {
		name    string
		msg     InboundMessage
		wantKey string
	}{
		{
			name:    "no override",
			msg:     InboundMessage{Channel: "telegram", ChatID: "123"},
			wantKey: "telegram:123",
		},
		{
			name:    "with override",
			msg:     InboundMessage{Channel: "telegram", ChatID: "123", SessionKeyOverride: "custom-key"},
			wantKey: "custom-key",
		},
		{
			name:    "empty channel and chatID",
			msg:     InboundMessage{},
			wantKey: ":",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.msg.SessionKey()
			if got != tc.wantKey {
				t.Errorf("SessionKey() = %q, want %q", got, tc.wantKey)
			}
		})
	}
}
