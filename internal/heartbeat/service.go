package heartbeat

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/coopco/nanobot/internal/bus"
	"github.com/coopco/nanobot/internal/providers"
)

type Service struct {
	provider  providers.Provider
	model     string
	bus       *bus.MessageBus
	workspace string
	interval  time.Duration
	onExecute func(ctx context.Context, message string)
	mu        sync.Mutex
	stopCh    chan struct{}
	running   bool
}

type Config struct {
	Provider  providers.Provider
	Model     string
	Bus       *bus.MessageBus
	Workspace string
	Interval  time.Duration
	OnExecute func(ctx context.Context, message string)
}

func NewService(cfg Config) *Service {
	interval := cfg.Interval
	if interval == 0 {
		interval = 30 * time.Minute
	}
	return &Service{
		provider:  cfg.Provider,
		model:     cfg.Model,
		bus:       cfg.Bus,
		workspace: cfg.Workspace,
		interval:  interval,
		onExecute: cfg.OnExecute,
		stopCh:    make(chan struct{}),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.tick(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

func (s *Service) TriggerNow(ctx context.Context) {
	s.tick(ctx)
}

var heartbeatToolDef = providers.ToolDef{
	Type: "function",
	Function: providers.FunctionDef{
		Name:        "heartbeat_decision",
		Description: "Decide whether to run the heartbeat action",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {"type": "string", "enum": ["skip", "run"]},
				"reason": {"type": "string"},
				"message": {"type": "string", "description": "Message to process if action is run"}
			},
			"required": ["action"]
		}`),
	},
}

type heartbeatDecision struct {
	Action  string `json:"action"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (s *Service) tick(ctx context.Context) {
	heartbeatPath := filepath.Join(s.workspace, "HEARTBEAT.md")
	data, err := os.ReadFile(heartbeatPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("heartbeat: HEARTBEAT.md not found, skipping")
			return
		}
		slog.Error("heartbeat: failed to read HEARTBEAT.md", "error", err)
		return
	}

	req := providers.ChatRequest{
		Model: s.model,
		Messages: []providers.Message{
			{Role: "user", Content: string(data)},
		},
		Tools: []providers.ToolDef{heartbeatToolDef},
	}

	resp, err := s.provider.Chat(ctx, req)
	if err != nil {
		slog.Error("heartbeat: phase 1 LLM call failed", "error", err)
		return
	}

	if len(resp.ToolCalls) == 0 {
		slog.Debug("heartbeat: no tool call in phase 1 response, skipping")
		return
	}

	var decision heartbeatDecision
	if err := json.Unmarshal([]byte(resp.ToolCalls[0].Arguments), &decision); err != nil {
		slog.Error("heartbeat: failed to parse decision", "error", err)
		return
	}

	switch decision.Action {
	case "skip":
		slog.Info("heartbeat: decision=skip", "reason", decision.Reason)
	case "run":
		slog.Info("heartbeat: decision=run", "reason", decision.Reason, "message", decision.Message)
		if s.onExecute != nil {
			s.onExecute(ctx, decision.Message)
		}
	default:
		slog.Warn("heartbeat: unknown action", "action", decision.Action)
	}
}
