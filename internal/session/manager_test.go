package session

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewSession(t *testing.T) {
	m := NewManager(t.TempDir())
	s := m.GetOrCreate("test:key")
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if len(s.Messages) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(s.Messages))
	}
	if s.Meta.Key != "test:key" {
		t.Fatalf("expected key 'test:key', got %q", s.Meta.Key)
	}
}

func TestAppendMessage(t *testing.T) {
	m := NewManager(t.TempDir())
	s := m.GetOrCreate("test:append")

	s.AppendMessage(Message{Role: "user", Content: "hello"})
	s.AppendMessage(Message{Role: "assistant", Content: "hi"})

	msgs := s.AllMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestGetHistory(t *testing.T) {
	m := NewManager(t.TempDir())
	s := m.GetOrCreate("test:history")

	for i := 0; i < 5; i++ {
		s.AppendMessage(Message{Role: "user", Content: fmt.Sprintf("msg%d", i)})
	}
	s.SetConsolidated(2)

	history := s.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 messages in history, got %d", len(history))
	}
	if history[0].Content != "msg2" {
		t.Errorf("expected first history message to be 'msg2', got %q", history[0].Content)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	s := m.GetOrCreate("telegram:99999")
	s.AppendMessage(Message{Role: "user", Content: "save me"})
	s.AppendMessage(Message{Role: "assistant", Content: "saved"})
	s.SetConsolidated(1)

	if err := m.Save(s); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load via a fresh manager (no cache)
	m2 := NewManager(dir)
	s2 := m2.GetOrCreate("telegram:99999")

	if s2.Meta.Key != "telegram:99999" {
		t.Errorf("expected key 'telegram:99999', got %q", s2.Meta.Key)
	}
	if s2.Meta.LastConsolidated != 1 {
		t.Errorf("expected LastConsolidated=1, got %d", s2.Meta.LastConsolidated)
	}
	msgs := s2.AllMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after load, got %d", len(msgs))
	}
	if msgs[0].Content != "save me" || msgs[1].Content != "saved" {
		t.Errorf("unexpected messages after load: %+v", msgs)
	}
}

func TestGetOrCreate(t *testing.T) {
	m := NewManager(t.TempDir())
	s1 := m.GetOrCreate("cache:test")
	s1.AppendMessage(Message{Role: "user", Content: "cached"})

	s2 := m.GetOrCreate("cache:test")
	if s1 != s2 {
		t.Error("expected same session pointer from cache")
	}
	if len(s2.AllMessages()) != 1 {
		t.Errorf("expected 1 message in cached session, got %d", len(s2.AllMessages()))
	}
}

func TestConcurrentAppend(t *testing.T) {
	m := NewManager(t.TempDir())
	s := m.GetOrCreate("concurrent:test")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.AppendMessage(Message{Role: "user", Content: fmt.Sprintf("msg%d", n)})
		}(i)
	}
	wg.Wait()

	if len(s.AllMessages()) != 50 {
		t.Errorf("expected 50 messages, got %d", len(s.AllMessages()))
	}
}

func TestSessionKeyFilename(t *testing.T) {
	cases := []struct {
		key      string
		expected string
	}{
		{"telegram:12345", "telegram_12345.jsonl"},
		{"channel/sub", "channel_sub.jsonl"},
		{"a:b/c", "a_b_c.jsonl"},
		{"plain", "plain.jsonl"},
	}
	for _, tc := range cases {
		got := keyToFilename(tc.key)
		if got != tc.expected {
			t.Errorf("keyToFilename(%q) = %q, want %q", tc.key, got, tc.expected)
		}
	}
}
