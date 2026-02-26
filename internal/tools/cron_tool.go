package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// CronManager defines the interface for managing cron jobs.
type CronManager interface {
	AddJob(schedule, message, sessionKey string) (string, error)
	RemoveJob(id string) error
	ListJobs() string
}

type ManageCronTool struct {
	manager CronManager
}

func NewManageCronTool(manager CronManager) *ManageCronTool {
	return &ManageCronTool{manager: manager}
}

func (t *ManageCronTool) Name() string        { return "manage_cron" }
func (t *ManageCronTool) Description() string { return "Add, remove, or list cron jobs" }
func (t *ManageCronTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["add", "remove", "list"],
				"description": "Action to perform"
			},
			"schedule": {
				"type": "string",
				"description": "Cron expression or interval (for add)"
			},
			"message": {
				"type": "string",
				"description": "Message to send (for add)"
			},
			"session_key": {
				"type": "string",
				"description": "Target session (for add)"
			},
			"job_id": {
				"type": "string",
				"description": "Job ID (for remove)"
			}
		},
		"required": ["action"]
	}`)
}

func (t *ManageCronTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action     string `json:"action"`
		Schedule   string `json:"schedule"`
		Message    string `json:"message"`
		SessionKey string `json:"session_key"`
		JobID      string `json:"job_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "add":
		if p.Schedule == "" || p.Message == "" || p.SessionKey == "" {
			return "", fmt.Errorf("schedule, message, and session_key are required for add action")
		}
		jobID, err := t.manager.AddJob(p.Schedule, p.Message, p.SessionKey)
		if err != nil {
			return "", fmt.Errorf("failed to add job: %w", err)
		}
		return fmt.Sprintf("Cron job added: %s", jobID), nil

	case "remove":
		if p.JobID == "" {
			return "", fmt.Errorf("job_id is required for remove action")
		}
		if err := t.manager.RemoveJob(p.JobID); err != nil {
			return "", fmt.Errorf("failed to remove job: %w", err)
		}
		return fmt.Sprintf("Cron job removed: %s", p.JobID), nil

	case "list":
		return t.manager.ListJobs(), nil

	default:
		return "", fmt.Errorf("invalid action: %s (must be add, remove, or list)", p.Action)
	}
}
