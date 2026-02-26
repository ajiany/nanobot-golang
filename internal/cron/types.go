package cron

import "time"

// ScheduleType defines how a cron job is scheduled.
type ScheduleType string

const (
	ScheduleAt    ScheduleType = "at"    // specific time (e.g. "14:30")
	ScheduleEvery ScheduleType = "every" // interval (e.g. "30m", "2h")
	ScheduleCron  ScheduleType = "cron"  // cron expression (e.g. "0 */2 * * *")
)

type CronSchedule struct {
	Type       ScheduleType `json:"type"`
	Expression string       `json:"expression"` // cron expr, time, or duration
}

type CronJob struct {
	ID         string       `json:"id"`
	Schedule   CronSchedule `json:"schedule"`
	Message    string       `json:"message"`    // message to send when triggered
	SessionKey string       `json:"sessionKey"` // target session
	CreatedAt  time.Time    `json:"createdAt"`
}

// CronStore persists jobs to a JSON file.
type CronStore struct {
	Jobs []CronJob `json:"jobs"`
}
