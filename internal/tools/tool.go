package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface all tools must implement
type Tool interface {
	Name() string
	Description() string
	Parameters() json.RawMessage // JSON Schema
	Execute(ctx context.Context, params json.RawMessage) (string, error)
}
