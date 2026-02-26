package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Message represents a single message in a session
type Message struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCallRecord `json:"tool_calls,omitempty"`
	Timestamp  string           `json:"timestamp,omitempty"`
}

// ToolCallRecord holds a single tool call within a message
type ToolCallRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SessionMeta is stored as the first line of the JSONL file
type SessionMeta struct {
	Key              string `json:"key"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	LastConsolidated int    `json:"last_consolidated"`
}

// Session holds conversation state
type Session struct {
	Meta     SessionMeta
	Messages []Message
	mu       sync.RWMutex
}

// AppendMessage adds a message (append-only, never delete)
func (s *Session) AppendMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg.Timestamp == "" {
		msg.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	s.Messages = append(s.Messages, msg)
	s.Meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
}

// GetHistory returns messages from LastConsolidated onwards (for LLM context)
func (s *Session) GetHistory() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	start := s.Meta.LastConsolidated
	if start >= len(s.Messages) {
		return []Message{}
	}
	result := make([]Message, len(s.Messages)-start)
	copy(result, s.Messages[start:])
	return result
}

// AllMessages returns all messages
func (s *Session) AllMessages() []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Message, len(s.Messages))
	copy(result, s.Messages)
	return result
}

// SetConsolidated updates the consolidation pointer
func (s *Session) SetConsolidated(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Meta.LastConsolidated = index
}

// Manager handles session persistence
type Manager struct {
	dataDir string
	cache   map[string]*Session
	mu      sync.RWMutex
}

// NewManager creates a Manager rooted at dataDir
func NewManager(dataDir string) *Manager {
	return &Manager{
		dataDir: dataDir,
		cache:   make(map[string]*Session),
	}
}

// keyToFilename replaces unsafe characters for use as a filename
func keyToFilename(key string) string {
	r := strings.NewReplacer(":", "_", "/", "_")
	return r.Replace(key) + ".jsonl"
}

// GetOrCreate returns existing session or creates a new one
func (m *Manager) GetOrCreate(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.cache[key]; ok {
		return s
	}

	s := m.load(key)
	if s == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		s = &Session{
			Meta: SessionMeta{
				Key:       key,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Messages: []Message{},
		}
	}
	m.cache[key] = s
	return s
}

// Save persists session to a JSONL file
func (m *Manager) Save(s *Session) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	path := filepath.Join(m.dataDir, keyToFilename(s.Meta.Key))
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(s.Meta); err != nil {
		return fmt.Errorf("failed to write session meta: %w", err)
	}
	for _, msg := range s.Messages {
		if err := enc.Encode(msg); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}
	return nil
}

// load reads a session from disk; returns nil if the file does not exist
func (m *Manager) load(key string) *Session {
	path := filepath.Join(m.dataDir, keyToFilename(key))
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// First line is SessionMeta
	if !scanner.Scan() {
		return nil
	}
	var meta SessionMeta
	if err := json.Unmarshal(scanner.Bytes(), &meta); err != nil {
		return nil
	}

	var messages []Message
	for scanner.Scan() {
		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	if messages == nil {
		messages = []Message{}
	}

	return &Session{Meta: meta, Messages: messages}
}
