package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"go.uber.org/zap"
)

func TestSchedulePollerCatchupExecutesMissedTicks(t *testing.T) {
	base := time.Now().Truncate(time.Minute)

	last := base.Add(-90 * time.Minute)
	sch := &schedule.WorkflowSchedule{
		ID:                "sch-1",
		AppID:             "app-1",
		WorkflowTriggerID: "trg-1",
		Cron:              "*/30 * * * *",
		TargetType:        schedule.TargetAll,
		Status:            "active",
		CreatedAt:         last,
		LastRunAt:         &last,
	}

	repo := &fakeScheduleRepo{items: []*schedule.WorkflowSchedule{sch}}
	wf := &fakeWorkflowService{}
	users := []*user.User{
		{UserID: "u1", AppID: "app-1"},
		{UserID: "u2", AppID: "app-1"},
	}
	userRepo := &fakeUserRepo{users: users}

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

	require.Equal(t, 3, wf.calls, "should trigger for each missed tick")
	require.NotNil(t, repo.updatedLastRun, "last_run_at should be updated")
	require.True(t, repo.updatedLastRun.After(base.Add(-1*time.Hour)))
	require.True(t, repo.updatedLastRun.Before(time.Now().Add(time.Minute)))
}

// --- fakes ---

type fakeScheduleRepo struct {
	items          []*schedule.WorkflowSchedule
	updatedLastRun *time.Time
}

func (f *fakeScheduleRepo) Create(ctx context.Context, s *schedule.WorkflowSchedule) error {
	return nil
}
func (f *fakeScheduleRepo) GetByID(ctx context.Context, id string) (*schedule.WorkflowSchedule, error) {
	return nil, nil
}
func (f *fakeScheduleRepo) List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*schedule.WorkflowSchedule, int64, error) {
	return nil, 0, nil
}
func (f *fakeScheduleRepo) ListDue(ctx context.Context, at time.Time) ([]*schedule.WorkflowSchedule, error) {
	return f.items, nil
}
func (f *fakeScheduleRepo) Update(ctx context.Context, s *schedule.WorkflowSchedule) error {
	f.updatedLastRun = s.LastRunAt
	return nil
}
func (f *fakeScheduleRepo) Delete(ctx context.Context, id string) error { return nil }

type fakeWorkflowService struct {
	calls int
}

func (f *fakeWorkflowService) Create(ctx context.Context, appID string, req *workflow.CreateRequest) (*workflow.Workflow, error) {
	return nil, nil
}
func (f *fakeWorkflowService) Get(ctx context.Context, id, appID string) (*workflow.Workflow, error) {
	return nil, nil
}
func (f *fakeWorkflowService) Update(ctx context.Context, id, appID string, req *workflow.UpdateRequest) (*workflow.Workflow, error) {
	return nil, nil
}
func (f *fakeWorkflowService) Delete(ctx context.Context, id, appID string) error { return nil }
func (f *fakeWorkflowService) List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*workflow.Workflow, int64, error) {
	return nil, 0, nil
}
func (f *fakeWorkflowService) Trigger(ctx context.Context, appID string, req *workflow.TriggerRequest) (*workflow.WorkflowExecution, error) {
	return nil, nil
}
func (f *fakeWorkflowService) TriggerByTopic(ctx context.Context, appID string, req *workflow.TriggerByTopicRequest) (*workflow.TriggerByTopicResult, error) {
	return nil, nil
}
func (f *fakeWorkflowService) TriggerForUserIDs(ctx context.Context, appID, triggerID string, userIDs []string, payload map[string]any) (*workflow.TriggerForUserIDsResult, error) {
	f.calls++
	return &workflow.TriggerForUserIDsResult{Triggered: len(userIDs)}, nil
}
func (f *fakeWorkflowService) CancelExecution(ctx context.Context, executionID, appID string) error {
	return nil
}
func (f *fakeWorkflowService) GetExecution(ctx context.Context, executionID, appID string) (*workflow.WorkflowExecution, error) {
	return nil, nil
}
func (f *fakeWorkflowService) ListExecutions(ctx context.Context, workflowID, appID string, limit, offset int) ([]*workflow.WorkflowExecution, int64, error) {
	return nil, 0, nil
}

type fakeTopicService struct{}

func (f *fakeTopicService) Create(ctx context.Context, appID string, req *topic.CreateRequest) (*topic.Topic, error) {
	return nil, nil
}
func (f *fakeTopicService) Get(ctx context.Context, id, appID string) (*topic.Topic, error) {
	return nil, nil
}
func (f *fakeTopicService) GetByID(ctx context.Context, id, appID string) (*topic.Topic, error) {
	return nil, nil
}
func (f *fakeTopicService) GetByKey(ctx context.Context, appID, key string) (*topic.Topic, error) {
	return nil, nil
}
func (f *fakeTopicService) List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*topic.Topic, int64, error) {
	return nil, 0, nil
}
func (f *fakeTopicService) Update(ctx context.Context, id, appID string, req *topic.UpdateRequest) (*topic.Topic, error) {
	return nil, nil
}
func (f *fakeTopicService) Delete(ctx context.Context, id, appID string) error { return nil }
func (f *fakeTopicService) Subscribe(ctx context.Context, appID, topicID, userID string) error {
	return nil
}
func (f *fakeTopicService) Unsubscribe(ctx context.Context, appID, topicID, userID string) error {
	return nil
}
func (f *fakeTopicService) AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error {
	return nil
}
func (f *fakeTopicService) RemoveSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error {
	return nil
}
func (f *fakeTopicService) GetSubscribers(ctx context.Context, topicID, appID string, limit, offset int) ([]topic.TopicSubscription, int64, error) {
	return nil, 0, nil
}
func (f *fakeTopicService) GetSubscriberUserIDs(ctx context.Context, topicID, appID string) ([]string, error) {
	return []string{}, nil
}

type fakeUserRepo struct {
	users []*user.User
}

func (f *fakeUserRepo) Create(ctx context.Context, u *user.User) error             { return nil }
func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) { return nil, nil }
func (f *fakeUserRepo) GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Update(ctx context.Context, u *user.User) error { return nil }
func (f *fakeUserRepo) List(ctx context.Context, filter user.UserFilter) ([]*user.User, error) {
	if filter.Offset >= len(f.users) {
		return []*user.User{}, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(f.users) || filter.Limit == 0 {
		end = len(f.users)
	}
	return f.users[filter.Offset:end], nil
}
func (f *fakeUserRepo) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeUserRepo) AddDevice(ctx context.Context, userID string, device user.Device) error {
	return nil
}
func (f *fakeUserRepo) RemoveDevice(ctx context.Context, userID, deviceID string) error { return nil }
func (f *fakeUserRepo) UpdatePreferences(ctx context.Context, userID string, preferences user.Preferences) error {
	return nil
}
func (f *fakeUserRepo) Count(ctx context.Context, filter user.UserFilter) (int64, error) {
	return int64(len(f.users)), nil
}
func (f *fakeUserRepo) BulkCreate(ctx context.Context, users []*user.User) error { return nil }
