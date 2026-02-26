package cron

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	robfigcron "github.com/robfig/cron/v3"

	"github.com/coopco/nanobot/internal/bus"
)

type Service struct {
	scheduler *robfigcron.Cron
	bus       *bus.MessageBus
	storePath string
	jobs      map[string]robfigcron.EntryID
	jobDefs   map[string]CronJob
	mu        sync.Mutex
	counter   int
}

func NewService(storePath string, msgBus *bus.MessageBus) *Service {
	return &Service{
		scheduler: robfigcron.New(),
		bus:       msgBus,
		storePath: storePath,
		jobs:      make(map[string]robfigcron.EntryID),
		jobDefs:   make(map[string]CronJob),
	}
}

// Start begins the cron scheduler.
func (s *Service) Start() {
	s.scheduler.Start()
}

// Stop stops the cron scheduler.
func (s *Service) Stop() {
	s.scheduler.Stop()
}

// AddJob adds a new cron job. Returns the job ID.
func (s *Service) AddJob(schedule CronSchedule, message, sessionKey string) (string, error) {
	cronExpr, err := toCronExpr(schedule)
	if err != nil {
		return "", fmt.Errorf("invalid schedule: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("cron_%d", s.counter)
	s.counter++

	job := CronJob{
		ID:        id,
		Schedule:  schedule,
		Message:   message,
		SessionKey: sessionKey,
		CreatedAt: time.Now(),
	}

	entryID, err := s.scheduler.AddFunc(cronExpr, func() {
		s.bus.PublishInbound(bus.InboundMessage{
			Channel:            "system",
			Content:            message,
			SessionKeyOverride: sessionKey,
			Metadata:           map[string]string{"source": "cron", "job_id": id},
		})
	})
	if err != nil {
		return "", fmt.Errorf("failed to register cron job: %w", err)
	}

	s.jobs[id] = entryID
	s.jobDefs[id] = job

	if err := s.saveToDisk(); err != nil {
		slog.Warn("failed to persist cron jobs", "error", err)
	}

	return id, nil
}

// RemoveJob removes a cron job by ID.
func (s *Service) RemoveJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entryID, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q not found", id)
	}

	s.scheduler.Remove(entryID)
	delete(s.jobs, id)
	delete(s.jobDefs, id)

	if err := s.saveToDisk(); err != nil {
		slog.Warn("failed to persist cron jobs after removal", "error", err)
	}

	return nil
}

// ListJobs returns all registered jobs.
func (s *Service) ListJobs() []CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]CronJob, 0, len(s.jobDefs))
	for _, job := range s.jobDefs {
		result = append(result, job)
	}
	return result
}

// LoadFromDisk loads persisted jobs and re-registers them.
func (s *Service) LoadFromDisk() error {
	data, err := os.ReadFile(s.storePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read cron store: %w", err)
	}

	var store CronStore
	if err := json.Unmarshal(data, &store); err != nil {
		return fmt.Errorf("failed to parse cron store: %w", err)
	}

	for _, job := range store.Jobs {
		if _, err := s.AddJob(job.Schedule, job.Message, job.SessionKey); err != nil {
			slog.Warn("failed to restore cron job", "id", job.ID, "error", err)
		}
	}
	return nil
}

// saveToDisk persists current jobs to JSON file. Caller must hold s.mu.
func (s *Service) saveToDisk() error {
	jobs := make([]CronJob, 0, len(s.jobDefs))
	for _, job := range s.jobDefs {
		jobs = append(jobs, job)
	}

	store := CronStore{Jobs: jobs}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cron store: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.storePath), 0o755); err != nil {
		return fmt.Errorf("failed to create store directory: %w", err)
	}

	return os.WriteFile(s.storePath, data, 0o644)
}

// toCronExpr converts a CronSchedule to a robfig/cron expression string.
func toCronExpr(schedule CronSchedule) (string, error) {
	switch schedule.Type {
	case ScheduleCron:
		return schedule.Expression, nil
	case ScheduleEvery:
		d, err := time.ParseDuration(schedule.Expression)
		if err != nil {
			return "", fmt.Errorf("invalid duration %q: %w", schedule.Expression, err)
		}
		return fmt.Sprintf("@every %s", d), nil
	case ScheduleAt:
		var h, m int
		if _, err := fmt.Sscanf(schedule.Expression, "%d:%d", &h, &m); err != nil {
			return "", fmt.Errorf("invalid time %q, expected HH:MM: %w", schedule.Expression, err)
		}
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return "", fmt.Errorf("time %q out of range", schedule.Expression)
		}
		return fmt.Sprintf("%d %d * * *", m, h), nil
	default:
		return "", fmt.Errorf("unknown schedule type %q", schedule.Type)
	}
}
