// This file exports TypeScript types and interfaces used throughout the application, ensuring type safety.

export interface Application {
    id: string;
    name: string;
    description: string;
    apiKey: string;
    settings: ApplicationSettings;
}

export interface ApplicationSettings {
    key: string;
    value: string;
}

export interface ApiResponse<T> {
    data: T;
    message: string;
    success: boolean;
}

export interface CreateApplicationRequest {
    name: string;
    description: string;
}

export interface UpdateApplicationRequest {
    name?: string;
    description?: string;
}