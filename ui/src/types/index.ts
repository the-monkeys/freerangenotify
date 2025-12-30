// ============= Application Types =============
export interface Application {
    app_id: string;
    app_name: string;
    description?: string;
    api_key: string;
    webhook_url?: string;
    settings?: ApplicationSettings;
    created_at?: string;
    updated_at?: string;
    // status is not in the DTO response shown, but was used in my AppDetail fake code. DTO has no status field.
}

export interface ApplicationSettings {
    rate_limit?: number;
    retry_attempts?: number;
    default_template?: string;
    enable_webhooks?: boolean;
    enable_analytics?: boolean;
    validation_url?: string;
    validation_config?: ValidationConfig;
    // Allow dynamic keys if backend supports it
    [key: string]: any;
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
    settings?: ApplicationSettings;
}

export interface UpdateApplicationRequest {
    app_name?: string;
    description?: string;
    webhook_url?: string;
    settings?: ApplicationSettings;
}

// ============= User Types =============
export interface User {
    user_id: string;
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
    quiet_hours?: any;
    dnd?: boolean;
    daily_limit?: number;
    categories?: Record<string, any>;
}

export interface CreateUserRequest {
    user_id?: string;
    email?: string;
    phone?: string;
    timezone?: string;
    language?: string;
    preferences?: UserPreferences;
}

export interface UpdateUserRequest {
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
export interface Notification {
    notification_id: string;
    app_id: string;
    user_id: string;
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    priority: 'low' | 'normal' | 'high' | 'critical';
    status: string;
    title: string;
    body: string;
    data?: Record<string, any>;
    template_id?: string;
    scheduled_at?: string;
    sent_at?: string;
    created_at: string;
    updated_at: string;
}

export interface NotificationRequest {
    user_id: string;
    channel: 'push' | 'email' | 'sms' | 'webhook' | 'in_app' | 'sse';
    priority: 'low' | 'normal' | 'high' | 'critical';
    title: string;
    body: string;
    data?: Record<string, any>;
    template_id?: string;
    webhook_url?: string;
}

export interface BulkNotificationRequest {
    user_ids: string[];
    channel: string;
    priority: string;
    title: string;
    body: string;
    data?: Record<string, any>;
    template_id?: string;
}

export interface UpdateNotificationStatusRequest {
    status: string;
    error_message?: string;
}

// ============= Template Types =============
export interface Template {
    id: string;
    app_id: string;
    name: string;
    description?: string;
    channel: string;
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
    subject?: string;
    body: string;
    variables?: string[];
    metadata?: Record<string, any>;
    locale?: string;
}

export interface UpdateTemplateRequest {
    description?: string;
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