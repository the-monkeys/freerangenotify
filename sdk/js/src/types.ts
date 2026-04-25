// ── Notification Types ──

export interface QuickSendParams {
    to: string;
    template?: string;
    subject?: string;
    body?: string;
    data?: Record<string, unknown>;
    channel?: string;
    priority?: 'low' | 'normal' | 'high' | 'critical';
    scheduledAt?: Date;
    /** Idempotency key to prevent duplicate sends on retry */
    idempotencyKey?: string;

    // Rich webhook content. Optional; only honored when the resolved channel
    // is webhook-like (webhook, discord, slack, teams).
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface SendResult {
    notification_id: string;
    status: string;
    user_id: string;
    channel: string;
}

export interface NotificationSendParams {
    user_id: string;
    channel?: string;
    priority?: string;
    title?: string;
    body?: string;
    data?: Record<string, unknown>;
    template_id?: string;
    category?: string;
    scheduled_at?: string;
    webhook_url?: string;
    webhook_target?: string;
    /** Idempotency key to prevent duplicate sends on retry */
    idempotency_key?: string;

    // Rich webhook fields. Optional; only honored when channel === 'webhook'
    // and the resolved provider supports the field. See README for the
    // per-provider capability matrix (Discord, Slack, Teams, generic).
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface BulkSendParams {
    user_ids: string[];
    channel?: string;
    priority?: string;
    title?: string;
    body?: string;
    data?: Record<string, unknown>;
    template_id?: string;
    category?: string;
    /** Idempotency key to prevent duplicate sends on retry */
    idempotency_key?: string;

    // Rich webhook fields. Same shape and capability matrix as
    // NotificationSendParams. The same rich content is delivered to every
    // user in user_ids.
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface BulkSendResult {
    sent: number;
    total: number;
    items: SendResult[];
}

export interface BroadcastParams {
    template: string;
    data?: Record<string, unknown>;
    channel?: string;
    priority?: string;
    /** Idempotency key to prevent duplicate sends on retry */
    idempotency_key?: string;

    // Rich webhook fields. Same shape and capability matrix as
    // NotificationSendParams.
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface BroadcastResult {
    total_sent: number;
    notifications: SendResult[];
}

export interface ContentAttachment {
    type: 'image' | 'video' | 'file' | 'audio';
    url: string;
    name?: string;
    mime_type?: string;
    size?: number;
    alt_text?: string;
}

export interface ContentAction {
    type: 'link' | 'submit' | 'dismiss';
    label: string;
    url?: string;
    value?: string;
    style?: 'primary' | 'danger' | 'default';
}

export interface ContentField {
    key: string;
    value: string;
    inline?: boolean;
}

export interface ContentMention {
    platform: 'discord' | 'slack' | 'teams';
    platform_id: string;
    display?: string;
}

export interface ContentPollChoice {
    label: string;
    emoji?: string;
}

export interface ContentPoll {
    question: string;
    choices: ContentPollChoice[];
    multi_select?: boolean;
    duration_hours?: number;
}

export interface ContentStyle {
    severity?: 'info' | 'success' | 'warning' | 'danger';
    color?: string;
}

export interface NotificationContent {
    title: string;
    body: string;
    data?: Record<string, unknown>;
    media_url?: string;
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface NotificationResponse {
    notification_id: string;
    app_id: string;
    user_id: string;
    channel: string;
    priority: string;
    status: string;
    content?: NotificationContent;
    template_id?: string;
    category?: string;
    scheduled_at?: string;
    sent_at?: string;
    delivered_at?: string;
    read_at?: string;
    failed_at?: string;
    snoozed_until?: string;
    archived_at?: string;
    error_message?: string;
    retry_count: number;
    created_at: string;
    updated_at: string;
}

export interface NotificationListResponse {
    notifications: NotificationResponse[];
    total: number;
    page: number;
    page_size: number;
}

export interface ListNotificationsOptions {
    userId?: string;
    appId?: string;
    channel?: string;
    status?: string;
    category?: string;
    priority?: string;
    page?: number;
    pageSize?: number;
    unreadOnly?: boolean;
}

// ── User Types ──

export interface CreateUserParams {
    full_name?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    external_id?: string;
    webhook_url?: string;
    preferences?: Partial<Preferences>;
}

export interface UpdateUserParams {
    full_name?: string;
    external_id?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    webhook_url?: string;
    preferences?: Partial<Preferences>;
}

export interface User {
    user_id: string;
    app_id: string;
    external_id: string;
    full_name: string;
    email: string;
    phone: string;
    timezone: string;
    language: string;
    webhook_url: string;
    preferences?: Preferences;
    devices?: Device[];
    created_at: string;
    updated_at: string;
}

export interface UserListResponse {
    users: User[];
    total_count: number;
    page: number;
    page_size: number;
}

export interface BulkCreateUsersResult {
    created: number;
    total: number;
    errors?: string[];
}

export interface BulkCreateUsersParams {
    users: CreateUserParams[];
    skip_existing?: boolean;
    upsert?: boolean;
}

export interface Preferences {
    email_enabled?: boolean;
    push_enabled?: boolean;
    sms_enabled?: boolean;
    slack_enabled?: boolean;
    discord_enabled?: boolean;
    whatsapp_enabled?: boolean;
    quiet_hours?: QuietHours;
    dnd?: boolean;
    categories?: Record<string, CategoryPreference>;
    daily_limit?: number;
}

export interface QuietHours {
    start: string;
    end: string;
}

export interface CategoryPreference {
    enabled: boolean;
    enabled_channels?: string[];
}

export interface AddDeviceParams {
    platform: string;
    token: string;
}

export interface Device {
    device_id: string;
    platform: string;
    active: boolean;
    registered_at: string;
}

// ── Template Types ──

export interface CreateTemplateParams {
    app_id: string;
    name: string;
    description?: string;
    channel: string;
    webhook_target?: string;
    subject?: string;
    body: string;
    variables?: string[];
    metadata?: Record<string, unknown>;
    locale?: string;
    created_by?: string;
}

export interface UpdateTemplateParams {
    description?: string;
    webhook_target?: string;
    subject?: string;
    body?: string;
    variables?: string[];
    metadata?: Record<string, unknown>;
    status?: string;
    locale?: string;
    updated_by?: string;
}

export interface CreateVersionParams {
    description?: string;
    subject?: string;
    body?: string;
    variables?: string[];
    metadata?: Record<string, unknown>;
    locale?: string;
    created_by?: string;
}

export interface CloneTemplateParams {
    app_id: string;
}

export interface Template {
    id: string;
    app_id: string;
    name: string;
    description: string;
    channel: string;
    webhook_target?: string;
    subject: string;
    body: string;
    variables: string[];
    metadata?: Record<string, unknown>;
    controls?: TemplateControl[];
    control_values?: ControlValues;
    version: number;
    status: string;
    locale: string;
    created_by: string;
    updated_by: string;
    created_at: string;
    updated_at: string;
}

export interface TemplateListResponse {
    templates: Template[];
    total: number;
    limit: number;
    offset: number;
}

export interface ListTemplatesOptions {
    appId?: string;
    channel?: string;
    name?: string;
    status?: string;
    locale?: string;
    limit?: number;
    offset?: number;
}

export interface TemplateDiff {
    from_version: number;
    to_version: number;
    changes: FieldChange[];
}

export interface FieldChange {
    field: string;
    from: unknown;
    to: unknown;
}

// ── Content Control Types ──

export interface TemplateControl {
    key: string;
    label: string;
    type: 'text' | 'textarea' | 'url' | 'color' | 'image' | 'number' | 'boolean' | 'select';
    default?: string;
    placeholder?: string;
    required?: boolean;
    options?: string[];
    group?: string;
    help_text?: string;
}

export type ControlValues = Record<string, unknown>;

export interface ControlsResponse {
    controls: TemplateControl[];
    control_values: ControlValues;
}

// ── Workflow Types ──

export interface CreateWorkflowParams {
    name: string;
    description?: string;
    trigger_id: string;
    steps: WorkflowStep[];
}

export interface UpdateWorkflowParams {
    name?: string;
    description?: string;
    steps?: WorkflowStep[];
    status?: string;
}

export interface TriggerWorkflowParams {
    trigger_id: string;
    user_id: string;
    payload?: Record<string, unknown>;
}

export interface Workflow {
    id: string;
    app_id: string;
    name: string;
    description: string;
    trigger_id: string;
    steps: WorkflowStep[];
    status: string;
    version: number;
    created_by: string;
    created_at: string;
    updated_at: string;
}

export interface WorkflowStep {
    id: string;
    type: 'channel' | 'delay' | 'digest' | 'condition' | 'noop';
    name?: string;
    channel?: string;
    template_id?: string;
    delay_duration?: string;
    digest_key?: string;
    digest_window?: string;
    condition?: StepCondition;
    config?: Record<string, unknown>;
}

export interface StepCondition {
    field: string;
    operator: string;
    value: unknown;
    on_true?: string;
    on_false?: string;
}

export interface WorkflowExecution {
    id: string;
    workflow_id: string;
    app_id: string;
    user_id: string;
    transaction_id: string;
    current_step_id: string;
    status: string;
    payload?: Record<string, unknown>;
    started_at: string;
    completed_at?: string;
    updated_at: string;
}

export interface WorkflowListResponse {
    workflows: Workflow[];
    total: number;
    page: number;
    page_size: number;
}

export interface ExecutionListResponse {
    executions: WorkflowExecution[];
    total: number;
    page: number;
    page_size: number;
}

// ── Topic Types ──

export interface CreateTopicParams {
    name: string;
    key: string;
    description?: string;
}

export interface Topic {
    id: string;
    app_id: string;
    name: string;
    key: string;
    description: string;
    created_at: string;
    updated_at: string;
}

export interface TopicListResponse {
    topics: Topic[];
    total: number;
    page: number;
    page_size: number;
}

export interface TopicSubscription {
    id: string;
    topic_id: string;
    user_id: string;
    created_at: string;
}

export interface SubscriberListResponse {
    subscribers: TopicSubscription[];
    total: number;
    page: number;
    page_size: number;
}

// ── Presence Types ──

export interface CheckInParams {
    user_id: string;
    webhook_url?: string;
}

// ── SSE Types ──

export interface SSENotification {
    notification_id: string;
    title: string;
    body: string;
    channel?: string;
    category?: string;
    status: string;
    data?: Record<string, unknown>;
    created_at: string;
}

export interface SSEConnectionOptions {
    onNotification: (notification: SSENotification) => void;
    onConnected?: () => void;
    onError?: (event: Event) => void;
    onUnreadCountChange?: (count: number) => void;
    onConnectionChange?: (connected: boolean) => void;
    subscriberHash?: string;
    autoReconnect?: boolean;
    reconnectInterval?: number;
}

export interface SSEConnection {
    close: () => void;
}
