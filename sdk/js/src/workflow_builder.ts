import type {
    CreateWorkflowParams,
    WorkflowStep,
    StepCondition,
} from './types';

// ── Condition Operators ──

export type ConditionOperator = 'eq' | 'neq' | 'contains' | 'gt' | 'lt' | 'exists' | 'not_read';

// ── Step Builder Interface ──

export interface StepBuilder {
    build(): WorkflowStep;
}

// ── Workflow Builder ──

/**
 * Fluent builder for constructing workflow definitions.
 *
 * ```ts
 * const wf = workflow('welcome-onboarding')
 *     .desc('Multi-step onboarding')
 *     .trigger('user.signup')
 *     .inApp({ template: 'welcome_inapp' })
 *     .delay('1h')
 *     .step(condition('steps.step_0.read', 'not_read')
 *         .onTrue(noop())
 *         .onFalse(emailStep('Follow-up').template('welcome_email'))
 *     )
 *     .digest('project_updates', { window: '24h', template: 'daily_digest' });
 * ```
 */
export class WorkflowBuilder {
    private _name: string;
    private _description = '';
    private _triggerId: string;
    private _steps: WorkflowStep[] = [];

    constructor(name: string) {
        this._name = name;
        this._triggerId = name;
    }

    /** Set workflow description. */
    desc(description: string): this {
        this._description = description;
        return this;
    }

    /** Override the trigger identifier (defaults to name). */
    trigger(triggerId: string): this {
        this._triggerId = triggerId;
        return this;
    }

    /** Append a step to the workflow. */
    step(builder: StepBuilder): this {
        const built = builder.build();
        built.id = `step_${this._steps.length}`;
        this._steps.push(built);
        return this;
    }

    // ── Convenience shorthand methods ──

    /** Shorthand: append an email step. */
    email(opts: { name?: string; template: string }): this {
        return this.step(channelStep('email', opts.name ?? 'email').template(opts.template));
    }

    /** Shorthand: append an SMS step. */
    sms(opts: { name?: string; template: string }): this {
        return this.step(channelStep('sms', opts.name ?? 'sms').template(opts.template));
    }

    /** Shorthand: append a push step. */
    push(opts: { name?: string; template: string }): this {
        return this.step(channelStep('push', opts.name ?? 'push').template(opts.template));
    }

    /** Shorthand: append an in-app (SSE) step. */
    inApp(opts: { name?: string; template: string }): this {
        return this.step(channelStep('sse', opts.name ?? 'in_app').template(opts.template));
    }

    /** Shorthand: append a delay step. */
    delay(duration: string): this {
        return this.step(delayStep(duration));
    }

    /** Shorthand: append a digest step. */
    digest(key: string, opts?: { window?: string; maxBatch?: number; template?: string }): this {
        let d = digestStep(key);
        if (opts?.window) d = d.window(opts.window);
        if (opts?.maxBatch) d = d.maxBatch(opts.maxBatch);
        if (opts?.template) d = d.template(opts.template);
        return this.step(d);
    }

    /** Validate the workflow definition. Throws on errors. */
    validate(): void {
        if (!this._name) throw new Error('Workflow name is required');
        if (!this._triggerId) throw new Error('Workflow trigger_id is required');
        if (this._steps.length === 0) throw new Error('Workflow must have at least one step');

        this._steps.forEach((s, i) => {
            if (!s.type) throw new Error(`Step ${i}: type is required`);
            if (s.type === 'channel' && !s.channel)
                throw new Error(`Step ${i}: channel step must specify a channel`);
            if (s.type === 'delay' && !s.delay_duration)
                throw new Error(`Step ${i}: delay step must specify a duration`);
            if (s.type === 'digest' && !s.digest_key)
                throw new Error(`Step ${i}: digest step must specify a digest key`);
        });
    }

    /** Build and validate into CreateWorkflowParams. */
    build(): CreateWorkflowParams {
        this.validate();
        return {
            name: this._name,
            description: this._description,
            trigger_id: this._triggerId,
            steps: this._steps,
        };
    }
}

// ── Factory function ──

/** Create a new workflow builder. */
export function workflow(name: string): WorkflowBuilder {
    return new WorkflowBuilder(name);
}

// ── Channel Step Builder ──

export class ChannelStepBuilder implements StepBuilder {
    private _name: string;
    private _channel: string;
    private _templateId = '';
    private _provider = '';
    private _skipCondition?: StepCondition;
    private _cfg: Record<string, unknown> = {};

    constructor(channel: string, name: string) {
        this._channel = channel;
        this._name = name;
    }

    template(templateId: string): this {
        this._templateId = templateId;
        return this;
    }

    withProvider(provider: string): this {
        this._provider = provider;
        return this;
    }

    skipIf(cond: ConditionStepBuilder): this {
        this._skipCondition = cond.buildCondition();
        return this;
    }

    config(key: string, value: unknown): this {
        this._cfg[key] = value;
        return this;
    }

    build(): WorkflowStep {
        const step: WorkflowStep = {
            id: '',
            name: this._name,
            type: 'channel',
            channel: this._channel,
            template_id: this._templateId,
        };
        if (this._provider) {
            step.config = { ...this._cfg, provider: this._provider };
        } else if (Object.keys(this._cfg).length > 0) {
            step.config = { ...this._cfg };
        }
        if (this._skipCondition) {
            step.condition = this._skipCondition;
        }
        return step;
    }
}

/** Create a channel delivery step. */
export function channelStep(channel: string, name: string): ChannelStepBuilder {
    return new ChannelStepBuilder(channel, name);
}

/** Create an email delivery step. */
export function emailStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('email', name);
}

/** Create an SMS delivery step. */
export function smsStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('sms', name);
}

/** Create a push notification delivery step. */
export function pushStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('push', name);
}

/** Create an in-app (SSE) delivery step. */
export function inAppStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('sse', name);
}

/** Create a webhook delivery step. */
export function webhookStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('webhook', name);
}

/** Create a Slack delivery step. */
export function slackStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('slack', name);
}

/** Create a Discord delivery step. */
export function discordStep(name: string): ChannelStepBuilder {
    return new ChannelStepBuilder('discord', name);
}

// ── Delay Step Builder ──

export class DelayStepBuilder implements StepBuilder {
    private _duration: string;

    constructor(duration: string) {
        this._duration = duration;
    }

    build(): WorkflowStep {
        return {
            id: '',
            name: 'delay',
            type: 'delay',
            delay_duration: this._duration,
        };
    }
}

/** Create a delay step with the given duration (e.g., "1h", "30m"). */
export function delayStep(duration: string): DelayStepBuilder {
    return new DelayStepBuilder(duration);
}

// ── Digest Step Builder ──

export class DigestStepBuilder implements StepBuilder {
    private _key: string;
    private _window = '';
    private _maxBatch = 0;
    private _templateId = '';

    constructor(key: string) {
        this._key = key;
    }

    window(w: string): this {
        this._window = w;
        return this;
    }

    maxBatch(max: number): this {
        this._maxBatch = max;
        return this;
    }

    template(templateId: string): this {
        this._templateId = templateId;
        return this;
    }

    build(): WorkflowStep {
        const step: WorkflowStep = {
            id: '',
            name: `digest_${this._key}`,
            type: 'digest',
            digest_key: this._key,
            template_id: this._templateId,
        };
        if (this._window || this._maxBatch > 0) {
            step.config = {};
            if (this._window) step.config.window = this._window;
            if (this._maxBatch > 0) step.config.max_batch = this._maxBatch;
        }
        return step;
    }
}

/** Create a digest step that accumulates events by key. */
export function digestStep(key: string): DigestStepBuilder {
    return new DigestStepBuilder(key);
}

// ── Condition Step Builder ──

export class ConditionStepBuilder implements StepBuilder {
    private _field: string;
    private _operator: ConditionOperator;
    private _value: unknown;
    private _onTrue?: StepBuilder;
    private _onFalse?: StepBuilder;

    constructor(field: string, operator: ConditionOperator, value?: unknown) {
        this._field = field;
        this._operator = operator;
        this._value = value;
    }

    onTrue(step: StepBuilder): this {
        this._onTrue = step;
        return this;
    }

    onFalse(step: StepBuilder): this {
        this._onFalse = step;
        return this;
    }

    buildCondition(): StepCondition {
        return {
            field: this._field,
            operator: this._operator,
            value: this._value,
        };
    }

    build(): WorkflowStep {
        const step: WorkflowStep = {
            id: '',
            name: 'condition',
            type: 'condition',
            condition: this.buildCondition(),
        };
        const cfg: Record<string, unknown> = {};
        if (this._onTrue) cfg.on_true = this._onTrue.build();
        if (this._onFalse) cfg.on_false = this._onFalse.build();
        if (Object.keys(cfg).length > 0) step.config = cfg;
        return step;
    }
}

/** Create a conditional branching step. */
export function condition(field: string, op: ConditionOperator, value?: unknown): ConditionStepBuilder {
    return new ConditionStepBuilder(field, op, value);
}

// ── Noop Step Builder ──

export class NoopStepBuilder implements StepBuilder {
    build(): WorkflowStep {
        return { id: '', name: 'noop', type: 'noop' };
    }
}

/** Create a no-operation step (used as a branch target). */
export function noop(): NoopStepBuilder {
    return new NoopStepBuilder();
}
