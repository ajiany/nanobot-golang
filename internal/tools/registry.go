package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type ToolDefinition struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

type FunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type Registry struct {
	tools map[string]Tool
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) string {
	t, ok := r.Get(name)
	if !ok {
		r.mu.RLock()
		names := make([]string, 0, len(r.tools))
		for n := range r.tools {
			names = append(names, n)
		}
		r.mu.RUnlock()
		return fmt.Sprintf("Unknown tool: %s. Available tools: %s", name, strings.Join(names, ", "))
	}
	result, err := t.Execute(ctx, args)
	if err != nil {
		return fmt.Sprintf("Error executing %s: %v\n\n[Analyze the error above and try a different approach.]", name, err)
	}
	return result
}

func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToolDefinition{
			Type: "function",
			Function: FunctionDef{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

func (r *Registry) Clone() *Registry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	clone := NewRegistry()
	for k, v := range r.tools {
		clone.tools[k] = v
	}
	return clone
}
