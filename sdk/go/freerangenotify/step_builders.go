package freerangenotify

// StepBuilder is the interface that all step builders implement.
type StepBuilder interface {
	Build() WorkflowStep
}

// ── Channel Step Builders ──

// ChannelStepBuilder creates a channel delivery step (email, sms, push, webhook, sse, etc.).
type ChannelStepBuilder struct {
	name       string
	channel    string
	templateID string
	provider   string
	skipIf     *StepCondition
	config     map[string]interface{}
}

// Email creates an email delivery step.
func Email(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "email"}
}

// SMS creates an SMS delivery step.
func SMS(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "sms"}
}

// Push creates a push notification delivery step.
func Push(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "push"}
}

// InApp creates an in-app (SSE) delivery step.
func InApp(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "sse"}
}

// Webhook creates a webhook delivery step.
func Webhook(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "webhook"}
}

// Slack creates a Slack delivery step.
func Slack(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "slack"}
}

// Discord creates a Discord delivery step.
func Discord(name string) *ChannelStepBuilder {
	return &ChannelStepBuilder{name: name, channel: "discord"}
}

// Template sets the template ID for this channel step.
func (b *ChannelStepBuilder) Template(templateID string) *ChannelStepBuilder {
	b.templateID = templateID
	return b
}

// Provider overrides the delivery provider for this step.
func (b *ChannelStepBuilder) Provider(provider string) *ChannelStepBuilder {
	b.provider = provider
	return b
}

// SkipIf attaches a skip condition to this step.
func (b *ChannelStepBuilder) SkipIf(cond *ConditionBuilder) *ChannelStepBuilder {
	built := cond.BuildCondition()
	b.skipIf = &built
	return b
}

// Config sets additional configuration for this step.
func (b *ChannelStepBuilder) Config(key string, value interface{}) *ChannelStepBuilder {
	if b.config == nil {
		b.config = make(map[string]interface{})
	}
	b.config[key] = value
	return b
}

// Build serializes the channel step builder into a WorkflowStep.
func (b *ChannelStepBuilder) Build() WorkflowStep {
	step := WorkflowStep{
		Name:       b.name,
		Type:       "channel",
		Channel:    b.channel,
		TemplateID: b.templateID,
	}
	if b.provider != "" {
		step.Config = map[string]interface{}{"provider": b.provider}
	}
	if b.config != nil {
		if step.Config == nil {
			step.Config = make(map[string]interface{})
		}
		for k, v := range b.config {
			step.Config[k] = v
		}
	}
	if b.skipIf != nil {
		step.Condition = b.skipIf
	}
	return step
}

// ── Delay Step ──

// DelayStepBuilder creates a time-delay step.
type DelayStepBuilder struct {
	duration string
}

// Delay creates a delay step with the given duration (e.g., "1h", "30m", "24h").
func Delay(duration string) *DelayStepBuilder {
	return &DelayStepBuilder{duration: duration}
}

// Build serializes the delay step builder into a WorkflowStep.
func (b *DelayStepBuilder) Build() WorkflowStep {
	return WorkflowStep{
		Name:          "delay",
		Type:          "delay",
		DelayDuration: b.duration,
	}
}

// ── Digest Step ──

// DigestStepBuilder creates a digest/batching step.
type DigestStepBuilder struct {
	key        string
	window     string
	maxBatch   int
	templateID string
}

// Digest creates a digest step that accumulates events by key.
func Digest(key string) *DigestStepBuilder {
	return &DigestStepBuilder{key: key}
}

// Window sets the digest window duration (e.g., "24h", "1h").
func (b *DigestStepBuilder) Window(window string) *DigestStepBuilder {
	b.window = window
	return b
}

// MaxBatch sets the maximum events per digest batch.
func (b *DigestStepBuilder) MaxBatch(max int) *DigestStepBuilder {
	b.maxBatch = max
	return b
}

// Template sets the template used to render the digest summary.
func (b *DigestStepBuilder) Template(templateID string) *DigestStepBuilder {
	b.templateID = templateID
	return b
}

// Build serializes the digest step builder into a WorkflowStep.
func (b *DigestStepBuilder) Build() WorkflowStep {
	step := WorkflowStep{
		Name:       "digest_" + b.key,
		Type:       "digest",
		DigestKey:  b.key,
		TemplateID: b.templateID,
	}
	if b.window != "" || b.maxBatch > 0 {
		step.Config = make(map[string]interface{})
		if b.window != "" {
			step.Config["window"] = b.window
		}
		if b.maxBatch > 0 {
			step.Config["max_batch"] = b.maxBatch
		}
	}
	return step
}

// ── Condition Step ──

// ConditionOperator defines comparison operators for workflow conditions.
type ConditionOperator string

const (
	// OpEquals checks field equality.
	OpEquals ConditionOperator = "eq"
	// OpNotEqual checks field inequality.
	OpNotEqual ConditionOperator = "neq"
	// OpContains checks if a field contains a substring.
	OpContains ConditionOperator = "contains"
	// OpGT checks if a field is greater than a value.
	OpGT ConditionOperator = "gt"
	// OpLT checks if a field is less than a value.
	OpLT ConditionOperator = "lt"
	// OpExists checks if a field exists.
	OpExists ConditionOperator = "exists"
	// OpNotRead checks if a notification has not been read.
	OpNotRead ConditionOperator = "not_read"
)

// ConditionBuilder creates a conditional branching step.
type ConditionBuilder struct {
	field    string
	operator ConditionOperator
	value    interface{}
	onTrue   StepBuilder
	onFalse  StepBuilder
}

// Condition creates a condition step that branches based on a field comparison.
func Condition(field string, op ConditionOperator, value ...interface{}) *ConditionBuilder {
	var val interface{}
	if len(value) > 0 {
		val = value[0]
	}
	return &ConditionBuilder{field: field, operator: op, value: val}
}

// OnTrue sets the step to execute when the condition is true.
func (b *ConditionBuilder) OnTrue(step StepBuilder) *ConditionBuilder {
	b.onTrue = step
	return b
}

// OnFalse sets the step to execute when the condition is false.
func (b *ConditionBuilder) OnFalse(step StepBuilder) *ConditionBuilder {
	b.onFalse = step
	return b
}

// BuildCondition serializes to a StepCondition (used internally by SkipIf).
func (b *ConditionBuilder) BuildCondition() StepCondition {
	return StepCondition{
		Field:    b.field,
		Operator: string(b.operator),
		Value:    b.value,
	}
}

// Build serializes the condition builder into a WorkflowStep.
func (b *ConditionBuilder) Build() WorkflowStep {
	step := WorkflowStep{
		Name: "condition",
		Type: "condition",
		Condition: &StepCondition{
			Field:    b.field,
			Operator: string(b.operator),
			Value:    b.value,
		},
	}
	cfg := make(map[string]interface{})
	if b.onTrue != nil {
		cfg["on_true"] = b.onTrue.Build()
	}
	if b.onFalse != nil {
		cfg["on_false"] = b.onFalse.Build()
	}
	if len(cfg) > 0 {
		step.Config = cfg
	}
	return step
}

// ── Noop Step ──

// NoopStepBuilder creates a no-operation step (used as a branch target in conditions).
type NoopStepBuilder struct{}

// Noop creates a noop step.
func Noop() *NoopStepBuilder {
	return &NoopStepBuilder{}
}

// Build serializes the noop step builder into a WorkflowStep.
func (b *NoopStepBuilder) Build() WorkflowStep {
	return WorkflowStep{
		Name: "noop",
		Type: "noop",
	}
}
