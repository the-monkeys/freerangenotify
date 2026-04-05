//go:build integration

package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	"go.uber.org/zap"
)

// These tests exercise the catch-up logic with real time progression and the
// unexported polling helpers. They run with the "integration" build tag because
// they rely on time-based behaviour and are slower than unit tests.

func TestIntegration_WorkflowCronCatchup(t *testing.T) {
	base := time.Now().Truncate(time.Minute)
	last := base.Add(-45 * time.Minute)
	sch := &schedule.WorkflowSchedule{
		ID:                "sch-int-1",
		AppID:             "app-int",
		WorkflowTriggerID: "trg-int",
		Cron:              "*/15 * * * *", // should fire 3 times in 45 minutes
		TargetType:        schedule.TargetAll,
		Status:            "active",
		CreatedAt:         last,
		LastRunAt:         &last,
	}

	repo := &fakeScheduleRepo{items: []*schedule.WorkflowSchedule{sch}}
	wf := &fakeWorkflowService{}
	userRepo := &fakeUserRepo{users: []*user.User{{UserID: "u1", AppID: "app-int"}}}

	poller := NewSchedulePoller(
		repo,
		wf,
		&fakeTopicService{},
		userRepo,
		zap.NewNop(),
		SchedulePollerConfig{
			CatchupWindowMinutes: 180,
			CatchupMaxRuns:       10,
		},
	)

	ctx := context.Background()
	poller.runTick(ctx)

	require.Equal(t, 3, wf.calls)
	require.NotNil(t, repo.updatedLastRun)
	require.True(t, repo.updatedLastRun.After(base.Add(-time.Hour)))
}

func TestIntegration_WorkflowDelayCatchup(t *testing.T) {
	q := &fakeWorkflowQueue{}
	engine := NewEngine(nil, nil, q, nil, zap.NewNop(), nil, 0, 10)

	now := time.Now()
	late := queue.WorkflowQueueItem{ExecutionID: "exec-int-1", StepID: "delay-next", EnqueuedAt: now}
	require.NoError(t, q.EnqueueWorkflowDelayed(context.Background(), late, now.Add(-2*time.Hour)))

	engine.processDelayedOnce(context.Background())

	require.Len(t, q.main, 1)
	require.Equal(t, "exec-int-1", q.main[0].ExecutionID)
}
