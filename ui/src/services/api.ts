import axios from 'axios';
import type {
  Application,
  User,
  Notification,
  Template,
  Device,
  CreateApplicationRequest,
  UpdateApplicationRequest,
  CreateUserRequest,
  UpdateUserRequest,
  NotificationRequest,
  BulkNotificationRequest,
  BroadcastNotificationRequest,
  UpdateNotificationStatusRequest,
  CreateTemplateRequest,
  UpdateTemplateRequest,
  RenderTemplateRequest,
  CreateTemplateVersionRequest,
  AddDeviceRequest,
  TemplateVersion,
} from '../types';

// Use environment variable for backend URL
// In development, Vite proxy handles routing (can use relative URLs)
// In production (Vercel), must use absolute backend URL
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL 
    ? `${import.meta.env.VITE_API_BASE_URL}/v1`
    : '/v1',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add JWT token to requests
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Handle token refresh on 401
api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      try {
        const refreshToken = localStorage.getItem('refresh_token');
        if (refreshToken) {
          const { data } = await api.post('/auth/refresh', {
            refresh_token: refreshToken,
          });

          localStorage.setItem('access_token', data.access_token);
          localStorage.setItem('refresh_token', data.refresh_token);

          originalRequest.headers.Authorization = `Bearer ${data.access_token}`;
          return api(originalRequest);
        }
      } catch (refreshError) {
        // Only redirect if we're in a browser environment
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        if (typeof window !== 'undefined') {
          window.location.href = '/login';
        }
        return Promise.reject(refreshError);
      }
    }

    return Promise.reject(error);
  }
);

// ============= Application APIs =============
interface ApiResponse<T> {
  data: T;
  success: boolean;
}

interface ApplicationListResponse {
  applications: Application[];
  total_count: number;
  limit: number;
  offset: number;
}

export const applicationsAPI = {
  list: async () => {
    const { data } = await api.get<ApiResponse<ApplicationListResponse>>('/apps/');
    return data.data.applications;
  },

  get: async (id: string) => {
    const { data } = await api.get<ApiResponse<Application>>(`/apps/${id}`);
    return data.data;
  },

  create: async (payload: CreateApplicationRequest) => {
    const { data } = await api.post<ApiResponse<Application>>('/apps/', payload);
    return data.data;
  },

  update: async (id: string, payload: UpdateApplicationRequest) => {
    const { data } = await api.put<ApiResponse<Application>>(`/apps/${id}`, payload);
    return data.data;
  },

  delete: async (id: string) => {
    await api.delete(`/apps/${id}`);
  },

  regenerateKey: async (id: string) => {
    const { data } = await api.post<ApiResponse<Application>>(`/apps/${id}/regenerate-key`, {});
    return data.data;
  },

  getSettings: async (id: string) => {
    const { data } = await api.get<ApiResponse<Record<string, any>>>(`/apps/${id}/settings`);
    return data.data;
  },

  updateSettings: async (id: string, settings: Record<string, any>) => {
    const { data } = await api.put<ApiResponse<Record<string, any>>>(`/apps/${id}/settings`, settings);
    return data.data;
  },
};

// Helper to get auth headers
const getAuthHeaders = (apiKey?: string) => {
  if (!apiKey) return {};
  return { 'Authorization': `Bearer ${apiKey}` };
};

// ============= User APIs =============
interface UserListResponse {
  users: User[];
  total_count: number;
  page: number;
  page_size: number;
}

export const usersAPI = {
  list: async (apiKey: string) => {
    const { data } = await api.get<ApiResponse<UserListResponse>>('/users/', {
      headers: getAuthHeaders(apiKey)
    });
    return data.data.users;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<User>>(`/users/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  create: async (apiKey: string, payload: CreateUserRequest) => {
    const { data } = await api.post<ApiResponse<User>>('/users/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  update: async (apiKey: string, id: string, payload: UpdateUserRequest) => {
    const { data } = await api.put<ApiResponse<User>>(`/users/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/users/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  // Device Management
  addDevice: async (apiKey: string, userId: string, payload: AddDeviceRequest) => {
    const { data } = await api.post<ApiResponse<Device>>(`/users/${userId}/devices`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  getDevices: async (apiKey: string, userId: string) => {
    const { data } = await api.get<ApiResponse<Device[]>>(`/users/${userId}/devices`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  removeDevice: async (apiKey: string, userId: string, deviceId: string) => {
    await api.delete(`/users/${userId}/devices/${deviceId}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  // Preferences Management
  updatePreferences: async (apiKey: string, userId: string, preferences: any) => {
    const { data } = await api.put<ApiResponse<any>>(`/users/${userId}/preferences`, preferences, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  getPreferences: async (apiKey: string, userId: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/users/${userId}/preferences`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },
};

// ============= Notification APIs =============
interface NotificationListResponse {
  notifications: Notification[];
  total: number;
  page: number;
  page_size: number;
}

export const notificationsAPI = {
  list: async (apiKey: string) => {
    // Note: This endpoint is currently NOT wrapped in success/data envelope in backend
    const { data } = await api.get<NotificationListResponse>('/notifications/', {
      headers: getAuthHeaders(apiKey)
    });
    return data.notifications;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<Notification>(`/notifications/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  send: async (apiKey: string, payload: NotificationRequest) => {
    const { data } = await api.post<Notification>('/notifications/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  sendBulk: async (apiKey: string, payload: BulkNotificationRequest) => {
    const { data } = await api.post('/notifications/bulk', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  broadcast: async (apiKey: string, payload: BroadcastNotificationRequest) => {
    const { data } = await api.post('/notifications/broadcast', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  updateStatus: async (apiKey: string, id: string, payload: UpdateNotificationStatusRequest) => {
    const { data } = await api.put<Notification>(`/notifications/${id}/status`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  cancel: async (apiKey: string, id: string) => {
    await api.delete(`/notifications/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  retry: async (apiKey: string, id: string) => {
    const { data } = await api.post<Notification>(`/notifications/${id}/retry`, {}, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
};

// ============= Template APIs =============
interface TemplateListResponse {
  templates: Template[];
  total: number;
  limit: number;
  offset: number;
}

export const templatesAPI = {
  list: async (apiKey: string) => {
    // Note: This endpoint is currently NOT wrapped in success/data envelope in backend
    const { data } = await api.get<TemplateListResponse>('/templates/', {
      headers: getAuthHeaders(apiKey)
    });
    return data.templates;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<Template>(`/templates/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  create: async (apiKey: string, payload: CreateTemplateRequest) => {
    const { data } = await api.post<Template>('/templates/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  update: async (apiKey: string, id: string, payload: UpdateTemplateRequest) => {
    const { data } = await api.put<Template>(`/templates/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/templates/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  render: async (apiKey: string, id: string, payload: RenderTemplateRequest) => {
    const { data } = await api.post(`/templates/${id}/render`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  createVersion: async (apiKey: string, appId: string, templateName: string, payload: CreateTemplateVersionRequest) => {
    const { data } = await api.post(`/templates/${appId}/${templateName}/versions`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getVersions: async (apiKey: string, appId: string, templateName: string) => {
    const { data } = await api.get<TemplateVersion[]>(`/templates/${appId}/${templateName}/versions`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
};

// ============= Admin APIs =============
interface QueueStats {
  [key: string]: number;
}

export const adminAPI = {
  getQueueStats: async () => {
    // Admin routes are currently public in backend
    const { data } = await api.get<{ stats: QueueStats }>('/admin/queues/stats');
    return data.stats;
  },

  listDLQ: async () => {
    const { data } = await api.get<{ items: any[] }>('/admin/queues/dlq');
    return data.items;
  }
};

export default api;