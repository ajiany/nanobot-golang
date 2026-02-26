package channels

import (
	"encoding/json"
	"testing"

	"github.com/coopco/nanobot/internal/bus"
)

func TestGetFactoryNotFound(t *testing.T) {
	_, ok := GetFactory("nonexistent-channel-xyz")
	if ok {
		t.Fatal("expected GetFactory to return false for unregistered channel")
	}
}

func TestRegisterOverwrite(t *testing.T) {
	const name = "test-overwrite"
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return &mockChannel{name: name + "-v1"}, nil
	})
	Register(name, func(cfg json.RawMessage, msgBus *bus.MessageBus) (Channel, error) {
		return &mockChannel{name: name + "-v2"}, nil
	})

	factory, ok := GetFactory(name)
	if !ok {
		t.Fatalf("expected factory for %q", name)
	}
	ch, err := factory(json.RawMessage(`{}`), nil)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if ch.Name() != name+"-v2" {
		t.Errorf("expected overwritten factory, got name %q", ch.Name())
	}
}

func TestRegisteredNamesIncludesBuiltins(t *testing.T) {
	names := RegisteredNames()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	builtins := []string{"telegram", "discord", "slack", "whatsapp", "feishu", "dingtalk"}
	for _, b := range builtins {
		if !nameSet[b] {
			t.Errorf("expected built-in channel %q to be registered", b)
		}
	}
}

func TestRegisteredNamesNotEmpty(t *testing.T) {
	names := RegisteredNames()
	if len(names) == 0 {
		t.Fatal("expected at least one registered channel")
	}
}
