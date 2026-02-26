package cron

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/coopco/nanobot/internal/bus"
)

func TestAddAndListJobs(t *testing.T) {
	svc := NewService(filepath.Join(t.TempDir(), "cron.json"), bus.NewMessageBus(10))

	id1, err := svc.AddJob(CronSchedule{Type: ScheduleCron, Expression: "0 * * * *"}, "msg1", "session1")
	if err != nil {
		t.Fatalf("AddJob 1: %v", err)
	}
	id2, err := svc.AddJob(CronSchedule{Type: ScheduleEvery, Expression: "5m"}, "msg2", "session2")
	if err != nil {
		t.Fatalf("AddJob 2: %v", err)
	}

	jobs := svc.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	ids := map[string]bool{id1: true, id2: true}
	for _, j := range jobs {
		if !ids[j.ID] {
			t.Errorf("unexpected job ID %q", j.ID)
		}
	}
}

func TestRemoveJob(t *testing.T) {
	svc := NewService(filepath.Join(t.TempDir(), "cron.json"), bus.NewMessageBus(10))

	id, err := svc.AddJob(CronSchedule{Type: ScheduleCron, Expression: "0 * * * *"}, "msg", "session")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	if err := svc.RemoveJob(id); err != nil {
		t.Fatalf("RemoveJob: %v", err)
	}

	if jobs := svc.ListJobs(); len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after removal, got %d", len(jobs))
	}

	if err := svc.RemoveJob(id); err == nil {
		t.Fatal("expected error removing non-existent job")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "cron.json")
	msgBus := bus.NewMessageBus(10)

	svc1 := NewService(storePath, msgBus)
	_, err := svc1.AddJob(CronSchedule{Type: ScheduleCron, Expression: "0 * * * *"}, "hello", "s1")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	_, err = svc1.AddJob(CronSchedule{Type: ScheduleEvery, Expression: "10m"}, "world", "s2")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	svc2 := NewService(storePath, msgBus)
	if err := svc2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}

	jobs := svc2.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 restored jobs, got %d", len(jobs))
	}
}

func TestCronScheduleConversion(t *testing.T) {
	cases := []struct {
		schedule CronSchedule
		wantErr  bool
	}{
		{CronSchedule{Type: ScheduleCron, Expression: "0 */2 * * *"}, false},
		{CronSchedule{Type: ScheduleEvery, Expression: "30m"}, false},
		{CronSchedule{Type: ScheduleEvery, Expression: "2h"}, false},
		{CronSchedule{Type: ScheduleAt, Expression: "14:30"}, false},
		{CronSchedule{Type: ScheduleAt, Expression: "00:00"}, false},
		{CronSchedule{Type: ScheduleEvery, Expression: "notaduration"}, true},
		{CronSchedule{Type: ScheduleAt, Expression: "25:00"}, true},
		{CronSchedule{Type: ScheduleAt, Expression: "badtime"}, true},
	}

	for _, tc := range cases {
		expr, err := toCronExpr(tc.schedule)
		if tc.wantErr {
			if err == nil {
				t.Errorf("schedule %+v: expected error, got expr %q", tc.schedule, expr)
			}
		} else {
			if err != nil {
				t.Errorf("schedule %+v: unexpected error: %v", tc.schedule, err)
			}
			if expr == "" {
				t.Errorf("schedule %+v: got empty expression", tc.schedule)
			}
		}
	}
}

func TestJobTrigger(t *testing.T) {
	msgBus := bus.NewMessageBus(10)
	svc := NewService(filepath.Join(t.TempDir(), "cron.json"), msgBus)
	svc.Start()
	defer svc.Stop()

	_, err := svc.AddJob(CronSchedule{Type: ScheduleEvery, Expression: "1s"}, "ping", "test-session")
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msg, err := msgBus.ConsumeInbound(ctx)
	if err != nil {
		t.Fatalf("no message received within timeout: %v", err)
	}

	if msg.Content != "ping" {
		t.Errorf("expected content %q, got %q", "ping", msg.Content)
	}
	if msg.SessionKeyOverride != "test-session" {
		t.Errorf("expected session %q, got %q", "test-session", msg.SessionKeyOverride)
	}
	if msg.Metadata["source"] != "cron" {
		t.Errorf("expected source=cron, got %q", msg.Metadata["source"])
	}
}
