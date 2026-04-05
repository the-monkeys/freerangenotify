package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"go.uber.org/zap"
)

// SchedulePollerConfig tunes catch-up behaviour.
type SchedulePollerConfig struct {
	CatchupWindowMinutes int
	CatchupMaxRuns       int
}

// SchedulePoller runs scheduled workflows every minute
type SchedulePoller struct {
	scheduleRepo schedule.Repository
	workflowSvc  workflow.Service
	topicSvc     topic.Service
	userRepo     user.Repository
	logger       *zap.Logger
	config       SchedulePollerConfig

	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewSchedulePoller creates a new schedule poller
func NewSchedulePoller(
	scheduleRepo schedule.Repository,
	workflowSvc workflow.Service,
	topicSvc topic.Service,
	userRepo user.Repository,
	logger *zap.Logger,
	config SchedulePollerConfig,
) *SchedulePoller {
	if config.CatchupWindowMinutes == 0 {
		config.CatchupWindowMinutes = 1440 // 24h default
	}
	if config.CatchupMaxRuns == 0 {
		config.CatchupMaxRuns = 100
	}
	return &SchedulePoller{
		scheduleRepo: scheduleRepo,
		workflowSvc:  workflowSvc,
		topicSvc:     topicSvc,
		userRepo:     userRepo,
		logger:       logger,
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// Start begins the schedule poller (runs every minute)
func (p *SchedulePoller) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.poll(ctx)
	p.logger.Info("Schedule poller started")
}

// Shutdown stops the schedule poller
func (p *SchedulePoller) Shutdown() {
	close(p.stopChan)
	p.wg.Wait()
	p.logger.Info("Schedule poller stopped")
}

func (p *SchedulePoller) poll(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	// Run once on startup after a short delay (align to minute boundary)
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
		p.runTick(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.runTick(ctx)
		}
	}
}

func (p *SchedulePoller) runTick(ctx context.Context) {
	now := time.Now()
	schedules, err := p.scheduleRepo.ListDue(ctx, now)
	if err != nil {
		p.logger.Error("Failed to list due schedules", zap.Error(err))
		return
	}

	for _, sch := range schedules {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		cronSpec, err := parser.Parse(sch.Cron)
		if err != nil {
			p.logger.Error("Invalid cron expression for schedule",
				zap.String("schedule_id", sch.ID),
				zap.String("cron", sch.Cron),
				zap.Error(err))
			continue
		}

		loc := time.UTC
		if sch.Timezone != "" {
			l, err := time.LoadLocation(sch.Timezone)
			if err != nil {
				p.logger.Error("Failed to load timezone for schedule",
					zap.String("schedule_id", sch.ID),
					zap.String("timezone", sch.Timezone),
					zap.Error(err))
				continue
			}
			loc = l
		}

		nowLoc := now.In(loc).Truncate(time.Minute)
		windowFloor := nowLoc.Add(-time.Duration(p.config.CatchupWindowMinutes) * time.Minute)
		cursor := sch.CreatedAt.In(loc)
		if sch.LastRunAt != nil {
			cursor = sch.LastRunAt.In(loc)
		}
		if cursor.Before(windowFloor) {
			cursor = windowFloor
		}

		next := cronSpec.Next(cursor)
		executed := 0
		var lastFired time.Time

		for (next.Equal(nowLoc) || next.Before(nowLoc)) && executed < p.config.CatchupMaxRuns {
			// Resolve user IDs and trigger
			userIDs, err := p.resolveUserIDs(ctx, sch)
			if err != nil {
				p.logger.Error("Failed to resolve users for schedule",
					zap.String("schedule_id", sch.ID),
					zap.Error(err))
				break
			}

			if len(userIDs) == 0 {
				p.logger.Info("Schedule has no recipients, skipping",
					zap.String("schedule_id", sch.ID),
					zap.String("name", sch.Name))
			} else {
				payload := sch.Payload
				if payload == nil {
					payload = make(map[string]any)
				}
				payload["schedule_id"] = sch.ID
				payload["scheduled_at"] = next.In(time.UTC).Format(time.RFC3339)

				result, err := p.workflowSvc.TriggerForUserIDs(ctx, sch.AppID, sch.WorkflowTriggerID, userIDs, payload)
				if err != nil {
					p.logger.Error("Failed to trigger workflow for schedule",
						zap.String("schedule_id", sch.ID),
						zap.String("trigger_id", sch.WorkflowTriggerID),
						zap.Error(err))
					break
				}

				p.logger.Info("Schedule executed",
					zap.String("schedule_id", sch.ID),
					zap.String("name", sch.Name),
					zap.Int("triggered", result.Triggered),
					zap.Time("scheduled_at", next.In(time.UTC)))
			}

			lastFired = next.In(time.UTC)
			executed++
			next = cronSpec.Next(next)
		}

		if !lastFired.IsZero() {
			p.updateLastRun(ctx, sch, lastFired)
		}
	}
}

func (p *SchedulePoller) resolveUserIDs(ctx context.Context, sch *schedule.WorkflowSchedule) ([]string, error) {
	switch sch.TargetType {
	case schedule.TargetTopic:
		if p.topicSvc == nil {
			return nil, fmt.Errorf("topics feature is not enabled")
		}
		return p.topicSvc.GetSubscriberUserIDs(ctx, sch.TopicID, sch.AppID)
	case schedule.TargetAll:
		var userIDs []string
		limit := 100
		offset := 0
		for {
			users, err := p.userRepo.List(ctx, user.UserFilter{
				AppID:  sch.AppID,
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				return nil, err
			}
			for _, u := range users {
				userIDs = append(userIDs, u.UserID)
			}
			if len(users) < limit {
				break
			}
			offset += limit
		}
		return userIDs, nil
	default:
		return nil, fmt.Errorf("unknown target_type: %s", sch.TargetType)
	}
}

func (p *SchedulePoller) updateLastRun(ctx context.Context, sch *schedule.WorkflowSchedule, firedAt time.Time) {
	sch.LastRunAt = &firedAt
	if err := p.scheduleRepo.Update(ctx, sch); err != nil {
		p.logger.Warn("Failed to update schedule last_run_at",
			zap.String("schedule_id", sch.ID),
			zap.Error(err))
	}
}
