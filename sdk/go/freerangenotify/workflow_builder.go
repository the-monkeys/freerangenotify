package freerangenotify

import "fmt"

// WorkflowBuilder constructs a workflow definition using a fluent API.
// The builder validates structure at construction time, serializes to the
// existing CreateWorkflowParams format, and calls existing REST endpoints.
//
// Example:
//
//	wf := frn.NewWorkflow("welcome-onboarding").
//	    Description("Onboard new users with welcome + follow-up").
//	    TriggerID("user.signup").
//	    Step(frn.Email("Welcome Email").Template("welcome_email")).
//	    Step(frn.Delay("1h")).
//	    Step(frn.Condition("steps.step_0.read", frn.OpNotRead).
//	        OnTrue(frn.Noop()).
//	        OnFalse(frn.Email("Follow-up").Template("followup_email")),
//	    )
type WorkflowBuilder struct {
	name        string
	description string
	triggerID   string
	steps       []WorkflowStep
}

// NewWorkflow creates a new workflow builder with the given name.
// The trigger ID defaults to the workflow name.
func NewWorkflow(name string) *WorkflowBuilder {
	return &WorkflowBuilder{name: name, triggerID: name}
}

// Description sets the workflow description.
func (b *WorkflowBuilder) Description(desc string) *WorkflowBuilder {
	b.description = desc
	return b
}

// TriggerID overrides the trigger identifier (defaults to name).
func (b *WorkflowBuilder) TriggerID(id string) *WorkflowBuilder {
	b.triggerID = id
	return b
}

// Step appends a step to the workflow.
func (b *WorkflowBuilder) Step(step StepBuilder) *WorkflowBuilder {
	built := step.Build()
	built.ID = fmt.Sprintf("step_%d", len(b.steps))
	b.steps = append(b.steps, built)
	return b
}

// Validate checks the workflow definition for structural errors.
func (b *WorkflowBuilder) Validate() error {
	if b.name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if b.triggerID == "" {
		return fmt.Errorf("workflow trigger_id is required")
	}
	if len(b.steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}
	for i, s := range b.steps {
		if s.Type == "" {
			return fmt.Errorf("step %d: type is required", i)
		}
		if s.Type == "channel" && s.Channel == "" {
			return fmt.Errorf("step %d: channel step must specify a channel", i)
		}
		if s.Type == "delay" && s.DelayDuration == "" {
			return fmt.Errorf("step %d: delay step must specify a duration", i)
		}
		if s.Type == "digest" && s.DigestKey == "" {
			return fmt.Errorf("step %d: digest step must specify a digest key", i)
		}
	}
	return nil
}

// Build serializes the builder into CreateWorkflowParams.
// Returns an error if validation fails.
func (b *WorkflowBuilder) Build() (*CreateWorkflowParams, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &CreateWorkflowParams{
		Name:        b.name,
		Description: b.description,
		TriggerID:   b.triggerID,
		Steps:       b.steps,
	}, nil
}

// MustBuild is like Build but panics on validation error.
func (b *WorkflowBuilder) MustBuild() *CreateWorkflowParams {
	params, err := b.Build()
	if err != nil {
		panic(fmt.Sprintf("workflow build error: %v", err))
	}
	return params
}
