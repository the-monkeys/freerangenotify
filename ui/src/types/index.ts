// This file exports TypeScript types and interfaces used throughout the application, ensuring type safety.

// ============= Application Types =============
export interface Application {
    id: string;
    name: string;
    description?: string;
    apiKey: string;
    settings?: ApplicationSettings;
    createdAt?: string;
    updatedAt?: string;
}

export interface ApplicationSettings {
    key: string;
    value: string;
}

export interface CreateApplicationRequest {
    name: string;
    description?: string;
}

export interface UpdateApplicationRequest {
    name?: string;
    description?: string;
}

// ============= User Types =============
export interface User {
    id: string;
    name: string;
    email: string;
    appId: string;
    devices?: Device[];
    preferences?: UserPreferences;
    createdAt?: string;
    updatedAt?: string;
}

export interface Device {
    id: string;
    type: 'ios' | 'android' | 'web';
    token: string;
    enabled: boolean;
    createdAt?: string;
}

export interface UserPreferences {
    notificationFrequency: 'realtime' | 'daily' | 'weekly';
    channels: ('email' | 'sms' | 'push' | 'telegram' | 'slack')[];
    timezone?: string;
}

export interface CreateUserRequest {
    name: string;
    email: string;
}

export interface UpdateUserRequest {
    name?: string;
    email?: string;
}

// ============= Device Types =============
export interface AddDeviceRequest {
    type: 'ios' | 'android' | 'web';
    token: string;
}

// ============= Notification Types =============
export interface Notification {
    id: string;
    appId: string;
    userId: string;
    title: string;
    body: string;
    channels: string[];
    status: 'pending' | 'sent' | 'failed' | 'delivered';
    metadata?: Record<string, any>;
    createdAt?: string;
    sentAt?: string;
}

export interface NotificationRequest {
    userId: string;
    title: string;
    body: string;
    channels: string[];
    metadata?: Record<string, any>;
}

export interface BulkNotificationRequest {
    userIds: string[];
    title: string;
    body: string;
    channels: string[];
    metadata?: Record<string, any>;
}

export interface UpdateNotificationStatusRequest {
    status: 'sent' | 'failed' | 'delivered';
}

// ============= Template Types =============
export interface Template {
    id: string;
    appId: string;
    name: string;
    content: string;
    channels: string[];
    variables?: string[];
    versions?: TemplateVersion[];
    createdAt?: string;
    updatedAt?: string;
}

export interface TemplateVersion {
    id: string;
    version: number;
    content: string;
    createdAt?: string;
}

export interface CreateTemplateRequest {
    name: string;
    content: string;
    channels: string[];
}

export interface UpdateTemplateRequest {
    name?: string;
    content?: string;
    channels?: string[];
}

export interface RenderTemplateRequest {
    variables: Record<string, any>;
}

export interface RenderTemplateResponse {
    renderedContent: string;
}

export interface CreateTemplateVersionRequest {
    content: string;
}

// ============= API Response Types =============
export interface ApiResponse<T> {
    data: T;
    message: string;
    success: boolean;
}