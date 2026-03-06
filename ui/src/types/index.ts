// ============= Application Types =============
export interface Application {
    app_id: string;
    app_name: string;
    description?: string;
    api_key: string;
    webhook_url?: string;
    webhooks?: Record<string, string>;
    settings?: ApplicationSettings;
    created_at?: string;
    updated_at?: string;
    // status is not in the DTO response shown, but was used in my AppDetail fake code. DTO has no status field.
}

// ============= Auth Types =============
export interface AdminUser {
    user_id: string;
    email: string;
    full_name: string;
    is_active: boolean;
    created_at: string;
    updated_at: string;
    last_login_at?: string;
}

export interface AuthResponse {
    user: AdminUser;
    access_token: string;
    refresh_token: string;
    expires_at: string;
}

export interface LoginRequest {
    email: string;
    password: string;
}

export interface RegisterRequest {
    email: string;
    password: string;
    full_name: string;
}

export interface ForgotPasswordRequest {
    email: string;
}

export interface ResetPasswordRequest {
    token: string;
    new_password: string;
}

export interface ChangePasswordRequest {
    old_password: string;
    new_password: string;
}

export interface ApplicationSettings {
    rate_limit?: number;
    retry_attempts?: number;
    default_template?: string;
    enable_webhooks?: boolean;
    enable_analytics?: boolean;
    validation_url?: string;
    validation_config?: ValidationConfig;
    email_config?: EmailConfig;
    daily_email_limit?: number;
    default_preferences?: DefaultPreferences;
    [key: string]: any;
}

export interface DefaultPreferences {
    email_enabled?: boolean;
    push_enabled?: boolean;
    sms_enabled?: boolean;
}

export interface EmailConfig {
    provider_type: 'system' | 'smtp' | 'sendgrid';
    smtp_config?: SMTPConfig;
    sendgrid_config?: SendGridConfig;
}

export interface SMTPConfig {
    host: string;
    port: number;
    username?: string;
    password?: string;
    from_email?: string;
    from_name?: string;
}

export interface SendGridConfig {
    api_key: string;
    from_email?: string;
    from_name?: string;
}

export interface ValidationConfig {
    method: string;
    token_placement: string;
    token_key: string;
    static_headers?: Record<string, string>;
}

export interface CreateApplicationRequest {
    app_name: string;
    description?: string;
    webhook_url?: string;
    webhooks?: Record<string, string>;
    settings?: ApplicationSettings;
}

export interface UpdateApplicationRequest {
    app_name?: string;
    description?: string;
    webhook_url?: string;
    webhooks?: Record<string, string>;
    settings?: ApplicationSettings;
}

// ============= User Types =============
export interface User {
    user_id: string;
    external_id?: string;
    app_id: string;
    email: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
    created_at?: string;
    updated_at?: string;
}

export interface Device {
    device_id: string;
    platform: 'ios' | 'android' | 'web';
    token: string;
    last_seen_at?: string;
    created_at?: string;
}

export interface UserPreferences {
    email_enabled?: boolean;
    push_enabled?: boolean;
    sms_enabled?: boolean;
    quiet_hours?: { enabled: boolean; start: string; end: string; timezone?: string };
    dnd?: boolean;
    daily_limit?: number;
    categories?: Record<string, any>;
}

export interface CreateUserRequest {
    user_id?: string;
    external_id?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
}

export interface UpdateUserRequest {
    external_id?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
}

// ============= Device Types =============
export interface AddDeviceRequest {
    platform: 'ios' | 'android' | 'web';
    token: string;
}

// ============= Notification Types =============
export interface Recurrence {
    cron_expression: string;
    end_date?: string;
    count?: number;
    current_count?: number;
}

export interface Notification {
    notification_id: string;
    app_id: string;
    user_id: string;
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    priority: 'low' | 'normal' | 'high' | 'critical';
    status: string;
    content: {
        title: string;
        body: string;
        data?: Record<string, any>;
    };
    template_id?: string;
    category?: string;
    metadata?: Record<string, any>;
    scheduled_at?: string;
    sent_at?: string;
    delivered_at?: string;
    read_at?: string;
    failed_at?: string;
    error_message?: string;
    retry_count?: number;
    snoozed_until?: string;
    archived_at?: string;
    created_at: string;
    updated_at: string;
    recurrence?: Recurrence;
}

export interface NotificationRequest {
    user_id: string;
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    priority: 'low' | 'normal' | 'high' | 'critical';
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
    webhook_url?: string;
    webhook_target?: string;
    scheduled_at?: string;
    recurrence?: Recurrence;
}

export interface BulkNotificationRequest {
    user_ids: string[];
    channel: string;
    priority: string;
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
}

export interface BroadcastNotificationRequest {
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    priority: 'low' | 'normal' | 'high' | 'critical';
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
    scheduled_at?: string;
}

export interface UpdateNotificationStatusRequest {
    status: string;
    error_message?: string;
}

// ============= Quick-Send Types =============
export interface QuickSendRequest {
    to: string;
    channel?: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    template?: string;
    subject?: string;
    body?: string;
    data?: Record<string, any>;
    priority?: 'low' | 'normal' | 'high' | 'critical';
    scheduled_at?: string;
    webhook_url?: string;
}

export interface QuickSendResponse {
    notification_id: string;
    status: string;
    user_id: string;
    channel: string;
    message: string;
}

// ============= Template Types =============
export interface Template {
    id: string;
    app_id: string;
    name: string;
    description?: string;
    channel: string;
    webhook_target?: string;
    subject?: string;
    body: string;
    variables?: string[];
    metadata?: Record<string, any>;
    version: number;
    status: string;
    locale?: string;
    created_at: string;
    updated_at: string;
}

export interface TemplateVersion {
    id: string;
    version: number;
    subject?: string;
    body: string;
    created_at: string;
}

export interface CreateTemplateRequest {
    app_id: string;
    name: string;
    description?: string;
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    webhook_target?: string;
    subject?: string;
    body: string;
    variables?: string[];
    metadata?: Record<string, any>;
    locale?: string;
}

export interface UpdateTemplateRequest {
    description?: string;
    webhook_target?: string;
    subject?: string;
    body?: string;
    variables?: string[];
    metadata?: Record<string, any>;
    status?: string;
    locale?: string;
}

export interface RenderTemplateRequest {
    data: Record<string, any>;
}

export interface RenderTemplateResponse {
    rendered_body: string;
}

export interface CreateTemplateVersionRequest {
    description?: string;
    subject?: string;
    body: string;
    variables?: string[];
    metadata?: Record<string, any>;
}

// ============= API Response Types =============
export interface ApiResponse<T> {
    data: T;
    message: string;
    success: boolean;
}

// ============= Provider Health Types =============
export interface ProviderHealth {
    name: string;
    channel: string;
    healthy: boolean;
    breaker_state: string; // closed, open, half-open
    latency_ms?: number;
    last_error?: string;
    last_error_at?: string;
}

// ============= System Stats Types =============
export interface SystemStats {
    total_apps: number;
    total_users: number;
    total_templates: number;
    total_workflows: number;
    notifications_today: number;
    notifications_this_week: number;
    success_rate: number;
}

// ============= DLQ Types =============
export interface DLQItem {
    notification_id: string;
    priority: string;
    reason: string;
    timestamp: string;
    retry_count: number;
}

// ============= Analytics Types =============
export interface ChannelAnalytics {
    channel: string;
    sent: number;
    delivered: number;
    failed: number;
    total: number;
    success_rate: number;
}

export interface DailyStat {
    date: string;
    count: number;
}

export interface AnalyticsSummary {
    period: string;
    total_sent: number;
    total_delivered: number;
    total_failed: number;
    total_pending: number;
    total_read: number;
    total_queued: number;
    total_processing: number;
    total_all: number;
    success_rate: number;
    total_users: number;
    total_templates: number;
    total_workflows: number;
    avg_latency_ms?: number;
    by_channel: ChannelAnalytics[];
    daily_breakdown: DailyStat[];
}

// ============= Workflow Types =============
export type WorkflowStepType = 'channel' | 'delay' | 'digest' | 'condition';
export type WorkflowStatus = 'draft' | 'active' | 'inactive';
export type ExecutionStatus = 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';
export type StepResultStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type ConditionOperator = 'eq' | 'neq' | 'contains' | 'gt' | 'lt' | 'exists' | 'not_read';

export interface StepCondition {
    field: string;
    operator: ConditionOperator;
    value: any;
}

export interface StepConfig {
    channel?: string;
    template_id?: string;
    provider?: string;
    duration?: string;
    digest_key?: string;
    window?: string;
    max_batch?: number;
    condition?: StepCondition;
}

export interface WorkflowStep {
    id: string;
    name: string;
    type: WorkflowStepType;
    order: number;
    config: StepConfig;
    on_success?: string;
    on_failure?: string;
    skip_if?: StepCondition;
}

export interface Workflow {
    id: string;
    app_id: string;
    environment_id?: string;
    name: string;
    description: string;
    trigger_id: string;
    steps: WorkflowStep[];
    status: WorkflowStatus;
    version: number;
    created_by: string;
    created_at: string;
    updated_at: string;
}

export interface CreateWorkflowRequest {
    name: string;
    description?: string;
    trigger_id: string;
    steps: Omit<WorkflowStep, 'id'>[];
}

export interface UpdateWorkflowRequest {
    name?: string;
    description?: string;
    trigger_id?: string;
    steps?: Omit<WorkflowStep, 'id'>[];
    status?: WorkflowStatus;
}

export interface TriggerWorkflowRequest {
    trigger_id: string;
    user_id: string;
    payload?: Record<string, any>;
    transaction_id?: string;
}

export interface StepResult {
    step_id: string;
    status: StepResultStatus;
    notification_id?: string;
    digest_count?: number;
    started_at?: string;
    completed_at?: string;
    error?: string;
}

export interface WorkflowExecution {
    id: string;
    workflow_id: string;
    app_id: string;
    user_id: string;
    transaction_id?: string;
    status: ExecutionStatus;
    payload: Record<string, any>;
    step_results: Record<string, StepResult>;
    started_at: string;
    completed_at?: string;
}

// ============= Digest Rule Types =============
export type DigestRuleStatus = 'active' | 'inactive';

export interface DigestRule {
    id: string;
    app_id: string;
    environment_id?: string;
    name: string;
    digest_key: string;
    window: string;
    channel: string;
    template_id: string;
    max_batch: number;
    status: DigestRuleStatus;
    created_at: string;
    updated_at: string;
}

export interface CreateDigestRuleRequest {
    name: string;
    digest_key: string;
    window: string;
    channel: string;
    template_id: string;
    max_batch?: number;
}

export interface UpdateDigestRuleRequest {
    name?: string;
    digest_key?: string;
    window?: string;
    channel?: string;
    template_id?: string;
    max_batch?: number;
    status?: DigestRuleStatus;
}

// ============= Topic Types =============
export interface Topic {
    id: string;
    app_id: string;
    environment_id?: string;
    name: string;
    key: string;
    description?: string;
    subscriber_count?: number;
    created_at: string;
    updated_at: string;
}

export interface CreateTopicRequest {
    name: string;
    key: string;
    description?: string;
}

export interface UpdateTopicRequest {
    name?: string;
    key?: string;
    description?: string;
}

export interface TopicSubscription {
    id: string;
    topic_id: string;
    app_id: string;
    user_id: string;
    created_at: string;
}

export interface TopicSubscribersRequest {
    user_ids: string[];
}

// ============= Team / RBAC Types =============
export type TeamRole = 'owner' | 'admin' | 'editor' | 'viewer';

export interface AppMembership {
    membership_id: string;
    app_id: string;
    user_id: string;
    user_email: string;
    role: TeamRole;
    invited_by: string;
    created_at: string;
    updated_at: string;
}

export interface InviteMemberRequest {
    email: string;
    role: Exclude<TeamRole, 'owner'>;
}

export interface UpdateRoleRequest {
    role: TeamRole;
}

// ============= Audit Log Types =============
export type AuditAction = 'create' | 'update' | 'delete' | 'send';
export type ActorType = 'user' | 'api_key' | 'system';

export interface AuditLog {
    audit_id: string;
    app_id: string;
    environment_id?: string;
    actor_id: string;
    actor_type: ActorType;
    action: AuditAction;
    resource: string;
    resource_id: string;
    changes: Record<string, any>;
    ip_address?: string;
    user_agent?: string;
    created_at: string;
}

export interface AuditLogFilters {
    app_id?: string;
    actor_id?: string;
    action?: AuditAction;
    resource?: string;
    from_date?: string;
    to_date?: string;
    limit?: number;
    offset?: number;
}

// ============= Environment Types =============
export type EnvironmentName = 'development' | 'staging' | 'production';

export interface Environment {
    id: string;
    app_id: string;
    name: string;
    slug: string;
    api_key: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
}

export interface CreateEnvironmentRequest {
    name: EnvironmentName;
}

export interface PromoteEnvironmentRequest {
    source_env_id: string;
    target_env_id: string;
    resources: string[];
}

// ============= Custom Provider Types =============
export interface CustomProvider {
    provider_id: string;
    name: string;
    channel: string;
    webhook_url: string;
    headers?: Record<string, string>;
    signing_key?: string;
    active: boolean;
    created_at: string;
}

export interface RegisterProviderRequest {
    name: string;
    channel: string;
    webhook_url: string;
    headers?: Record<string, string>;
}

// ============= Presence Types =============
export interface PresenceCheckInRequest {
    user_id: string;
    url: string;
}

// ============= Batch Notification Types =============
export interface BatchNotificationRequest {
    notifications: NotificationRequest[];
}

export interface CancelBatchRequest {
    batch_id: string;
}

// ============= Notification Inbox Types =============
export interface MarkReadRequest {
    notification_ids: string[];
}

export interface MarkAllReadRequest {
    user_id: string;
}

export interface BulkArchiveRequest {
    notification_ids: string[];
}

export interface SnoozeRequest {
    until: string;
}

export interface UnreadCountResponse {
    user_id: string;
    count: number;
}

// ============= Template Advanced Types =============
export interface TemplateRollbackRequest {
    target_version: number;
}

export interface TemplateDiffResponse {
    from_version: number;
    to_version: number;
    changes: Record<string, { old: any; new: any }>;
}

export interface TemplateTestRequest {
    user_id: string;
    variables?: Record<string, any>;
}

export interface ContentControl {
    key: string;
    label: string;
    type: 'text' | 'textarea' | 'url' | 'color' | 'image' | 'number' | 'boolean' | 'select';
    default?: any;
    placeholder?: string;
    help_text?: string;
    group?: string;
    options?: string[];
}

export interface TemplateControlsResponse {
    controls: ContentControl[];
    values: Record<string, any>;
}

export interface UpdateControlsRequest {
    control_values: Record<string, any>;
}

// ============= User Advanced Types =============
export interface BulkCreateUsersRequest {
    users: CreateUserRequest[];
}

export interface SubscriberHashResponse {
    user_id: string;
    subscriber_hash: string;
}