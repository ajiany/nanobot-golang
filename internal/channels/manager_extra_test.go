package channels

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func TestAddChannelUnknown(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)

	err := mgr.AddChannel("no-such-channel-xyz", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown channel name")
	}
}

func TestStartAllAndStopAll(t *testing.T) {
	const name = "test-start-stop"
	mock := &mockChannel{name: name}
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return mock, nil
	})

	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.AddChannel(name, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("AddChannel: %v", err)
	}

	if err := mgr.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	if !mock.started {
		t.Error("expected channel to be started")
	}

	if err := mgr.StopAll(); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
}

func TestStartAllEmpty(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.StartAll(context.Background()); err != nil {
		t.Fatalf("StartAll on empty manager: %v", err)
	}
}

func TestStopAllEmpty(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.StopAll(); err != nil {
		t.Fatalf("StopAll on empty manager: %v", err)
	}
}

func TestNewManagerHasBus(t *testing.T) {
	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if mgr.bus != msgBus {
		t.Error("expected manager to hold the provided bus")
	}
}

func TestOutboundDispatchViaBus(t *testing.T) {
	const name = "test-bus-dispatch"
	mock := &mockChannel{name: name}
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return mock, nil
	})

	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.AddChannel(name, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("AddChannel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go msgBus.DispatchOutbound(ctx)

	msgBus.PublishOutbound(bus.OutboundMessage{Channel: name, Type: "text", Content: "dispatched"})

	// Wait for async dispatch
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		mgr.mu.Lock()
		n := len(mock.sent)
		mgr.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 message sent via bus, got %d", len(mock.sent))
	}
	if mock.sent[0].Content != "dispatched" {
		t.Errorf("expected content %q, got %q", "dispatched", mock.sent[0].Content)
	}
}

func TestOutboundDispatchProgressFiltered(t *testing.T) {
	const name = "test-filter-progress"
	mock := &mockChannel{name: name}
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return mock, nil
	})

	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.AddChannel(name, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("AddChannel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go msgBus.DispatchOutbound(ctx)

	msgBus.PublishOutbound(bus.OutboundMessage{Channel: name, Type: "progress", Content: "loading"})
	msgBus.PublishOutbound(bus.OutboundMessage{Channel: name, Type: "tool_hint", Content: "hint"})
	// sentinel to know both filtered messages were processed
	msgBus.PublishOutbound(bus.OutboundMessage{Channel: name, Type: "text", Content: "sentinel"})

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		mgr.mu.Lock()
		n := len(mock.sent)
		mgr.mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if len(mock.sent) != 1 {
		t.Fatalf("expected only sentinel message, got %d messages", len(mock.sent))
	}
	if mock.sent[0].Content != "sentinel" {
		t.Errorf("expected sentinel, got %q", mock.sent[0].Content)
	}
}

func TestOutboundDispatchWrongChannel(t *testing.T) {
	const name = "test-wrong-channel"
	mock := &mockChannel{name: name}
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return mock, nil
	})

	msgBus := bus.NewMessageBus(16)
	mgr := NewManager(msgBus)
	if err := mgr.AddChannel(name, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("AddChannel: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go msgBus.DispatchOutbound(ctx)

	// publish to a different channel name — mock should not receive it
	msgBus.PublishOutbound(bus.OutboundMessage{Channel: "other-channel", Type: "text", Content: "nope"})

	time.Sleep(50 * time.Millisecond)

	if len(mock.sent) != 0 {
		t.Errorf("expected 0 messages for wrong channel, got %d", len(mock.sent))
	}
}
