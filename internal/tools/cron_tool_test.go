package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mockCronManager implements CronManager for testing.
type mockCronManager struct {
	jobs    map[string]string // id -> description
	nextID  int
	addErr  error
	rmErr   error
}

func newMockCronManager() *mockCronManager {
	return &mockCronManager{jobs: make(map[string]string)}
}

func (m *mockCronManager) AddJob(schedule, message, sessionKey string) (string, error) {
	if m.addErr != nil {
		return "", m.addErr
	}
	m.nextID++
	id := fmt.Sprintf("job-%d", m.nextID)
	m.jobs[id] = fmt.Sprintf("%s|%s|%s", schedule, message, sessionKey)
	return id, nil
}

func (m *mockCronManager) RemoveJob(id string) error {
	if m.rmErr != nil {
		return m.rmErr
	}
	if _, ok := m.jobs[id]; !ok {
		return fmt.Errorf("job %s not found", id)
	}
	delete(m.jobs, id)
	return nil
}

func (m *mockCronManager) ListJobs() string {
	if len(m.jobs) == 0 {
		return "no jobs"
	}
	var sb strings.Builder
	for id, desc := range m.jobs {
		fmt.Fprintf(&sb, "%s: %s\n", id, desc)
	}
	return sb.String()
}

func TestManageCronTool_Add(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{
		"action":      "add",
		"schedule":    "*/5 * * * *",
		"message":     "ping",
		"session_key": "tg:123",
	})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Cron job added") {
		t.Errorf("unexpected result: %s", result)
	}
	if len(mgr.jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(mgr.jobs))
	}
}

func TestManageCronTool_Add_MissingFields(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)

	tests := []struct {
		name   string
		params map[string]any
	}{
		{"missing schedule", map[string]any{"action": "add", "message": "m", "session_key": "k"}},
		{"missing message", map[string]any{"action": "add", "schedule": "* * * * *", "session_key": "k"}},
		{"missing session_key", map[string]any{"action": "add", "schedule": "* * * * *", "message": "m"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(tt.params)
			_, err := tool.Execute(context.Background(), params)
			if err == nil {
				t.Fatal("expected error for missing required field")
			}
		})
	}
}

func TestManageCronTool_Remove(t *testing.T) {
	mgr := newMockCronManager()
	mgr.jobs["job-1"] = "test"
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "remove", "job_id": "job-1"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Cron job removed") {
		t.Errorf("unexpected result: %s", result)
	}
	if len(mgr.jobs) != 0 {
		t.Errorf("expected 0 jobs after remove, got %d", len(mgr.jobs))
	}
}

func TestManageCronTool_Remove_MissingJobID(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "remove"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for missing job_id")
	}
}

func TestManageCronTool_Remove_ManagerError(t *testing.T) {
	mgr := newMockCronManager()
	mgr.rmErr = fmt.Errorf("remove failed")
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "remove", "job_id": "job-1"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error from manager")
	}
}

func TestManageCronTool_List(t *testing.T) {
	mgr := newMockCronManager()
	mgr.jobs["job-1"] = "schedule|msg|key"
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "list"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "job-1") {
		t.Errorf("expected job-1 in list: %s", result)
	}
}

func TestManageCronTool_List_Empty(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "list"})
	result, err := tool.Execute(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result for list")
	}
}

func TestManageCronTool_InvalidAction(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{"action": "bogus"})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestManageCronTool_InvalidParams(t *testing.T) {
	mgr := newMockCronManager()
	tool := NewManageCronTool(mgr)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not-json`))
	if err == nil {
		t.Fatal("expected error for invalid params")
	}
}

func TestManageCronTool_Add_ManagerError(t *testing.T) {
	mgr := newMockCronManager()
	mgr.addErr = fmt.Errorf("add failed")
	tool := NewManageCronTool(mgr)

	params, _ := json.Marshal(map[string]any{
		"action":      "add",
		"schedule":    "* * * * *",
		"message":     "m",
		"session_key": "k",
	})
	_, err := tool.Execute(context.Background(), params)
	if err == nil {
		t.Fatal("expected error from manager")
	}
}

func TestManageCronTool_Name(t *testing.T) {
	tool := NewManageCronTool(newMockCronManager())
	if tool.Name() != "manage_cron" {
		t.Errorf("Name() = %q, want manage_cron", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() is empty")
	}
	if len(tool.Parameters()) == 0 {
		t.Error("Parameters() is empty")
	}
}
