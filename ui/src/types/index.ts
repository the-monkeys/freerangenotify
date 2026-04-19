// ============= Application Types =============
export interface Application {
    app_id: string;
    app_name: string;
    description?: string;
    api_key: string;
    admin_user_id?: string;
    tenant_id?: string;
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
    phone?: string;
    phone_verified?: boolean;
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
    whatsapp_config?: WhatsAppConfig;
    sms_config?: SMSConfig;
    default_preferences?: DefaultPreferences;
    on_user_created_trigger_id?: string;  // Phase 5: workflow to trigger on user create
    inbound_webhook_config?: InboundWebhookConfig;  // Phase 7: inbound webhooks
    [key: string]: any;
}

export interface InboundWebhookConfig {
    secret?: string;
    event_mapping?: Record<string, string>;  // event -> workflow trigger_id
}

export interface DefaultPreferences {
    email_enabled?: boolean;
    push_enabled?: boolean;
    sms_enabled?: boolean;
    whatsapp_enabled?: boolean;
}

export interface EmailConfig {
    provider_type: 'system' | 'smtp' | 'sendgrid';
    smtp?: SMTPConfig;
    sendgrid?: SendGridConfig;
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

export interface WhatsAppConfig {
    account_sid?: string;
    auth_token?: string;
    from_number?: string;
    // Meta Cloud API fields (populated via Embedded Signup)
    provider?: 'twilio' | 'meta';
    meta_phone_number_id?: string;
    meta_waba_id?: string;
    meta_access_token?: string;
    meta_business_id?: string;
    connection_status?: 'connected' | 'disconnected' | '';
    connected_at?: string;
    display_phone_number?: string;
    quality_rating?: string;
}

// ============= WhatsApp Meta Types =============
export interface WhatsAppConnectionStatus {
    connected: boolean;
    provider: string;
    connection_status?: string;
    connected_at?: string;
    phone_number_id?: string;
    waba_id?: string;
    display_phone?: string;
    quality_rating?: string;
    business_id?: string;
    message?: string;
}

export interface WhatsAppMetaTemplate {
    id: string;
    name: string;
    language: string;
    status: string;
    category: string;
    components?: any[];
}

export interface WhatsAppConversation {
    contact_wa_id: string;
    contact_name?: string;
    last_message?: string;
    last_message_at?: string;
    unread_count?: number;
    csw_open?: boolean;
}

export interface WhatsAppMessage {
    id: string;
    app_id: string;
    meta_message_id?: string;
    contact_wa_id: string;
    contact_name?: string;
    direction: 'inbound' | 'outbound';
    message_type: string;
    body?: string;
    media_url?: string;
    timestamp: string;
    status?: string;
    metadata?: Record<string, any>;
}

// ============= Twilio Content Template Types =============
export interface TwilioContentTemplate {
    sid: string;
    friendly_name: string;
    language: string;
    types: Record<string, any>;
    variables?: Record<string, string>;
    date_created: string;
    date_updated: string;
    approval_requests?: {
        name?: string;
        status?: string;
        category?: string;
        rejection_reason?: string;
        content_type?: string;
        allow_category_change?: boolean;
    };
}

export interface SMSConfig {
    account_sid: string;
    auth_token: string;
    from_number: string;
}

export interface ValidationConfig {
    method: string;
    token_placement: string;
    token_key: string;
    static_headers?: Record<string, string>;
}

export interface CreateApplicationRequest {
    tenant_id?: string;
    app_name: string;
    description?: string;
    webhook_url?: string;
    webhooks?: Record<string, string>;
    settings?: ApplicationSettings;
}

export interface UpdateApplicationRequest {
    app_name?: string;
    description?: string;
    tenant_id?: string;
    webhook_url?: string;
    webhooks?: Record<string, string>;
    settings?: ApplicationSettings;
}

// ============= User Types =============
export interface User {
    user_id: string;
    external_id?: string;
    full_name?: string;
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
    full_name?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
}

export interface UpdateUserRequest {
    external_id?: string;
    full_name?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
}

export interface BulkCreateUserRequest {
    upsert: boolean;
    skip_existing: boolean;
    users: CreateUserRequest[];
}

export interface BulkUserError {
    index: number;
    email?: string;
    message: string;
}

export interface BulkCreateUserResponse {
    created: number;
    updated: number;
    skipped: number;
    total: number;
    errors?: BulkUserError[];
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
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse' | 'whatsapp';
    priority: 'low' | 'normal' | 'high' | 'critical';
    status: string;
    content: {
        title: string;
        body: string;
        data?: Record<string, any>;
        media_url?: string;
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
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse' | 'whatsapp';
    priority: 'low' | 'normal' | 'high' | 'critical';
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
    webhook_url?: string;
    webhook_target?: string;
    scheduled_at?: string;
    recurrence?: Recurrence;
    workflow_trigger_id?: string;  // Phase 3: trigger workflow after send
    metadata?: Record<string, any>;  // Digest: { digest_key }
    media_url?: string;
}

export interface BulkNotificationRequest {
    user_ids: string[];
    channel: string;
    priority: string;
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
    metadata?: Record<string, any>;  // Digest: { digest_key }
}

export interface BroadcastNotificationRequest {
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse' | 'whatsapp';
    priority: 'low' | 'normal' | 'high' | 'critical';
    title?: string;
    body?: string;
    data?: Record<string, any>;
    template_id?: string;
    scheduled_at?: string;
    workflow_trigger_id?: string;  // Phase 2: trigger workflow for each recipient
    topic_key?: string;           // Phase 2: limit to topic subscribers
    metadata?: Record<string, any>;  // Digest: { digest_key }
}

export interface UpdateNotificationStatusRequest {
    status: string;
    error_message?: string;
}

// ============= Quick-Send Types =============
export interface QuickSendRequest {
    to: string;
    channel?: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse' | 'whatsapp';
    template?: string;
    subject?: string;
    body?: string;
    data?: Record<string, any>;
    priority?: 'low' | 'normal' | 'high' | 'critical';
    scheduled_at?: string;
    webhook_url?: string;
    digest_key?: string;  // Batch via digest rule
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
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse' | 'whatsapp';
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
    editable?: boolean;
}

export interface RenderTemplateResponse {
    rendered_body: string;
    attribute_variables?: AttributeVar[];
}

export interface AttributeVar {
    name: string;
    type: 'image' | 'url' | 'attribute';
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

export interface TriggerByTopicRequest {
    trigger_id: string;
    topic_id: string;
    payload?: Record<string, any>;
}

export interface TriggerByTopicResult {
    triggered: number;
    execution_ids: string[];
}

// ============= Workflow Schedule Types (Phase 6) =============
export type ScheduleTargetType = 'all' | 'topic';

export interface WorkflowSchedule {
    id: string;
    app_id: string;
    environment_id?: string;
    name: string;
    workflow_trigger_id: string;
    cron: string;
    timezone?: string;
    target_type: ScheduleTargetType;
    topic_id?: string;
    payload?: Record<string, any>;
    status: 'active' | 'inactive';
    last_run_at?: string;
    created_at: string;
    updated_at: string;
}

export interface CreateScheduleRequest {
    name: string;
    workflow_trigger_id: string;
    cron: string;
    timezone?: string;
    target_type: ScheduleTargetType;
    topic_id?: string;
    payload?: Record<string, any>;
}

export interface UpdateScheduleRequest {
    name?: string;
    workflow_trigger_id?: string;
    cron?: string;
    timezone?: string;
    target_type?: ScheduleTargetType;
    topic_id?: string;
    payload?: Record<string, any>;
    status?: 'active' | 'inactive';
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
    on_subscribe_trigger_id?: string;  // Phase 4: workflow to trigger on subscribe
    subscriber_count?: number;
    created_at: string;
    updated_at: string;
}

export interface CreateTopicRequest {
    name: string;
    key: string;
    description?: string;
    on_subscribe_trigger_id?: string;  // Phase 4: workflow to trigger on subscribe
}

export interface UpdateTopicRequest {
    name?: string;
    key?: string;
    description?: string;
    on_subscribe_trigger_id?: string;  // Phase 4: workflow to trigger on subscribe
}

export interface TopicSubscription {
    id: string;
    topic_id: string;
    app_id: string;
    user_id: string;
    email?: string;
    full_name?: string;
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

// ============= Dashboard Notification Types =============
export interface DashboardNotification {
    id: string;
    user_id: string;
    title: string;
    body: string;
    category: string;
    data?: Record<string, unknown>;
    read_at?: string;
    created_at: string;
}

// ============= Tenant/Organization Types (C1) =============
export interface Tenant {
    id: string;
    name: string;
    slug: string;
    created_by: string;
    billing_tier?: string;
    license_key?: string;
    valid_until?: string;
    max_apps?: number;
    max_throughput?: number;
    created_at: string;
    updated_at: string;
}

export interface TenantBilling {
    billing_tier: string;
    valid_until: string;
    max_apps: number;
    max_throughput: number;
}

export interface TenantMember {
    id: string;
    tenant_id: string;
    user_id: string;
    user_email: string;
    role: 'owner' | 'admin' | 'member';
    invited_by: string;
    created_at: string;
    updated_at: string;
}

export interface CreateTenantRequest {
    name: string;
}

export interface InviteTenantMemberRequest {
    email: string;
    role: 'admin' | 'member';
}

// ============= Billing Types =============
export interface BillingSubscription {
    id: string;
    tenant_id: string;
    plan: string;
    status: 'trial' | 'active' | 'expired' | 'canceled';
    current_period_start: string;
    current_period_end: string;
    metadata?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
}

export interface BillingUsage {
    plan: string;
    status: 'trial' | 'active' | 'expired' | 'canceled' | 'none';
    messages_sent: number;
    message_limit: number;
    usage_percent: number;
    current_period_start: string;
    current_period_end: string;
    days_remaining: number;
}

export interface AcceptTrialResponse {
    accepted: boolean;
    plan: string;
    status: string;
    message_limit: number;
    current_period_end: string;
    days_remaining: number;
    trial_accepted_at: string;
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
    /** Notification IDs to cancel (backend expects this) */
    notification_ids: string[];
    /** @deprecated Use notification_ids. Batch ID from send response - no longer returned by API. */
    batch_id?: string;
}

// ============= Notification Inbox Types =============
export interface MarkReadRequest {
    notification_ids: string[];
    user_id: string;
}

export interface MarkAllReadRequest {
    user_id: string;
}

export interface BulkArchiveRequest {
    notification_ids: string[];
    user_id: string;
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
    version?: number;
}

export interface TemplateDiffChange {
    field: string;
    from: any;
    to: any;
}

export interface TemplateDiffResponse {
    from_version: number;
    to_version: number;
    changes: TemplateDiffChange[] | Record<string, { old: any; new: any }>;
}

export interface TemplateTestRequest {
    to_email: string;
    sample_data?: Record<string, any>;
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
    values?: Record<string, any>;
    control_values?: Record<string, any>;
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
