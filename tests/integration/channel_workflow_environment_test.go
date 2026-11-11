package integration

import (
	"context"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"strings"
	"testing"
)

func TestChannelWorkflowTriggerScopeRealES(t *testing.T) {
	client := channelFixture(t)
	res, err := client.Indices.Create("workflows", client.Indices.Create.WithBody(strings.NewReader(`{"mappings":{"properties":{"app_id":{"type":"keyword"},"trigger_id":{"type":"keyword"},"environment_id":{"type":"keyword"}}}}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	repo := repository.NewWorkflowRepository(client, zap.NewNop())
	app := uuid.NewString()
	trigger := uuid.NewString()
	ctx := context.Background()
	for _, env := range []string{"", "stage"} {
		wf := &workflow.Workflow{ID: uuid.NewString(), AppID: app, TriggerID: trigger, EnvironmentID: env, Status: workflow.WorkflowStatusActive}
		if err := repo.CreateWorkflow(ctx, wf); err != nil {
			t.Fatal(err)
		}
		defer repo.DeleteWorkflow(ctx, wf.ID)
	}
	for _, env := range []string{"default", "stage", "missing"} {
		wf, err := repo.GetWorkflowByTrigger(channelconnection.WithEnvironment(ctx, env), app, trigger)
		if env == "missing" {
			if err == nil {
				t.Fatal("unknown environment selected another workflow")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if env == "stage" && wf.EnvironmentID != "stage" || env == "default" && wf.EnvironmentID != "" {
			t.Fatalf("wrong workflow for %s: %s", env, wf.EnvironmentID)
		}
	}
}
