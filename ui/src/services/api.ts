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
  UpdateNotificationStatusRequest,
  CreateTemplateRequest,
  UpdateTemplateRequest,
  RenderTemplateRequest,
  CreateTemplateVersionRequest,
  AddDeviceRequest,
  TemplateVersion,
} from '../types';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';
const api = axios.create({
  baseURL: `${API_BASE_URL}/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
});

// ============= Application APIs =============
export const applicationsAPI = {
  list: async () => {
    const { data } = await api.get<Application[]>('/apps/');
    return data;
  },
  
  get: async (id: string) => {
    const { data } = await api.get<Application>(`/apps/${id}`);
    return data;
  },
  
  create: async (payload: CreateApplicationRequest) => {
    const { data } = await api.post<Application>('/apps/', payload);
    return data;
  },
  
  update: async (id: string, payload: UpdateApplicationRequest) => {
    const { data } = await api.put<Application>(`/apps/${id}`, payload);
    return data;
  },
  
  delete: async (id: string) => {
    await api.delete(`/apps/${id}`);
  },
  
  regenerateKey: async (id: string) => {
    const { data } = await api.post<Application>(`/apps/${id}/regenerate-key`, {});
    return data;
  },
  
  getSettings: async (id: string) => {
    const { data } = await api.get(`/apps/${id}/settings`);
    return data;
  },
  
  updateSettings: async (id: string, settings: Record<string, any>) => {
    const { data } = await api.put(`/apps/${id}/settings`, settings);
    return data;
  },
};

// ============= User APIs =============
export const usersAPI = {
  list: async () => {
    const { data } = await api.get<User[]>('/users/');
    return data;
  },
  
  get: async (id: string) => {
    const { data } = await api.get<User>(`/users/${id}`);
    return data;
  },
  
  create: async (payload: CreateUserRequest) => {
    const { data } = await api.post<User>('/users/', payload);
    return data;
  },
  
  update: async (id: string, payload: UpdateUserRequest) => {
    const { data } = await api.put<User>(`/users/${id}`, payload);
    return data;
  },
  
  delete: async (id: string) => {
    await api.delete(`/users/${id}`);
  },
  
  // Device Management
  addDevice: async (userId: string, payload: AddDeviceRequest) => {
    const { data } = await api.post<Device>(`/users/${userId}/devices`, payload);
    return data;
  },
  
  getDevices: async (userId: string) => {
    const { data } = await api.get<Device[]>(`/users/${userId}/devices`);
    return data;
  },
  
  removeDevice: async (userId: string, deviceId: string) => {
    await api.delete(`/users/${userId}/devices/${deviceId}`);
  },
  
  // Preferences Management
  updatePreferences: async (userId: string, preferences: any) => {
    const { data } = await api.put(`/users/${userId}/preferences`, preferences);
    return data;
  },
  
  getPreferences: async (userId: string) => {
    const { data } = await api.get(`/users/${userId}/preferences`);
    return data;
  },
};

// ============= Notification APIs =============
export const notificationsAPI = {
  list: async () => {
    const { data } = await api.get<Notification[]>('/notifications/');
    return data;
  },
  
  get: async (id: string) => {
    const { data } = await api.get<Notification>(`/notifications/${id}`);
    return data;
  },
  
  send: async (payload: NotificationRequest) => {
    const { data } = await api.post<Notification>('/notifications/', payload);
    return data;
  },
  
  sendBulk: async (payload: BulkNotificationRequest) => {
    const { data } = await api.post('/notifications/bulk', payload);
    return data;
  },
  
  updateStatus: async (id: string, payload: UpdateNotificationStatusRequest) => {
    const { data } = await api.put<Notification>(`/notifications/${id}/status`, payload);
    return data;
  },
  
  cancel: async (id: string) => {
    await api.delete(`/notifications/${id}`);
  },
  
  retry: async (id: string) => {
    const { data } = await api.post<Notification>(`/notifications/${id}/retry`, {});
    return data;
  },
};

// ============= Template APIs =============
export const templatesAPI = {
  list: async () => {
    const { data } = await api.get<Template[]>('/templates/');
    return data;
  },
  
  get: async (id: string) => {
    const { data } = await api.get<Template>(`/templates/${id}`);
    return data;
  },
  
  create: async (payload: CreateTemplateRequest) => {
    const { data } = await api.post<Template>('/templates/', payload);
    return data;
  },
  
  update: async (id: string, payload: UpdateTemplateRequest) => {
    const { data } = await api.put<Template>(`/templates/${id}`, payload);
    return data;
  },
  
  delete: async (id: string) => {
    await api.delete(`/templates/${id}`);
  },
  
  render: async (id: string, payload: RenderTemplateRequest) => {
    const { data } = await api.post(`/templates/${id}/render`, payload);
    return data;
  },
  
  createVersion: async (appId: string, templateName: string, payload: CreateTemplateVersionRequest) => {
    const { data } = await api.post(`/templates/${appId}/${templateName}/versions`, payload);
    return data;
  },
  
  getVersions: async (appId: string, templateName: string) => {
    const { data } = await api.get<TemplateVersion[]>(`/templates/${appId}/${templateName}/versions`);
    return data;
  },
};

export default api;