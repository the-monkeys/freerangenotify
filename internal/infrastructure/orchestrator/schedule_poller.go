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

// SchedulePoller runs scheduled workflows every minute
type SchedulePoller struct {
	scheduleRepo schedule.Repository
	workflowSvc  workflow.Service
	topicSvc     topic.Service
	userRepo     user.Repository
	logger       *zap.Logger

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
) *SchedulePoller {
	return &SchedulePoller{
		scheduleRepo: scheduleRepo,
		workflowSvc:  workflowSvc,
		topicSvc:     topicSvc,
		userRepo:     userRepo,
		logger:       logger,
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
	now := time.Now().UTC().Truncate(time.Minute)

	schedules, err := p.scheduleRepo.ListDue(ctx, now)
	if err != nil {
		p.logger.Error("Failed to list due schedules", zap.Error(err))
		return
	}

	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for _, sch := range schedules {
		parsed, err := cronParser.Parse(sch.Cron)
		if err != nil {
			p.logger.Warn("Invalid cron in schedule, skipping",
				zap.String("schedule_id", sch.ID),
				zap.String("cron", sch.Cron),
				zap.Error(err))
			continue
		}

		// Check if this schedule should run at `now`
		// Next(prevMinute) should equal now for a match
		prevMinute := now.Add(-1 * time.Minute)
		nextRun := parsed.Next(prevMinute)
		if !nextRun.Equal(now) {
			continue
		}

		// Resolve user IDs and trigger
		userIDs, err := p.resolveUserIDs(ctx, sch)
		if err != nil {
			p.logger.Error("Failed to resolve users for schedule",
				zap.String("schedule_id", sch.ID),
				zap.Error(err))
			continue
		}

		if len(userIDs) == 0 {
			p.logger.Info("Schedule has no recipients, skipping",
				zap.String("schedule_id", sch.ID),
				zap.String("name", sch.Name))
			p.updateLastRun(ctx, sch)
			continue
		}

		payload := sch.Payload
		if payload == nil {
			payload = make(map[string]any)
		}
		payload["schedule_id"] = sch.ID
		payload["scheduled_at"] = now.Format(time.RFC3339)

		result, err := p.workflowSvc.TriggerForUserIDs(ctx, sch.AppID, sch.WorkflowTriggerID, userIDs, payload)
		if err != nil {
			p.logger.Error("Failed to trigger workflow for schedule",
				zap.String("schedule_id", sch.ID),
				zap.String("trigger_id", sch.WorkflowTriggerID),
				zap.Error(err))
			continue
		}

		p.logger.Info("Schedule executed",
			zap.String("schedule_id", sch.ID),
			zap.String("name", sch.Name),
			zap.Int("triggered", result.Triggered))

		p.updateLastRun(ctx, sch)
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

func (p *SchedulePoller) updateLastRun(ctx context.Context, sch *schedule.WorkflowSchedule) {
	now := time.Now()
	sch.LastRunAt = &now
	if err := p.scheduleRepo.Update(ctx, sch); err != nil {
		p.logger.Warn("Failed to update schedule last_run_at",
			zap.String("schedule_id", sch.ID),
			zap.Error(err))
	}
}
