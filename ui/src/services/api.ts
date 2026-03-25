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
  QuickSendRequest,
  QuickSendResponse,
  ProviderHealth,
  DLQItem,
  AnalyticsSummary,
  Workflow,
  CreateWorkflowRequest,
  UpdateWorkflowRequest,
  TriggerWorkflowRequest,
  TriggerByTopicRequest,
  TriggerByTopicResult,
  WorkflowExecution,
  WorkflowSchedule,
  CreateScheduleRequest,
  UpdateScheduleRequest,
  DigestRule,
  CreateDigestRuleRequest,
  UpdateDigestRuleRequest,
  Topic,
  CreateTopicRequest,
  UpdateTopicRequest,
  TopicSubscription,
  TopicSubscribersRequest,
  AppMembership,
  InviteMemberRequest,
  UpdateRoleRequest,
  AuditLog,
  AuditLogFilters,
  Environment,
  CreateEnvironmentRequest,
  PromoteEnvironmentRequest,
  DashboardNotification,
  Tenant,
  TenantMember,
  CreateTenantRequest,
  InviteTenantMemberRequest,
  CustomProvider,
  RegisterProviderRequest,
  PresenceCheckInRequest,
  BatchNotificationRequest,
  CancelBatchRequest,
  MarkReadRequest,
  MarkAllReadRequest,
  BulkArchiveRequest,
  SnoozeRequest,
  UnreadCountResponse,
  TemplateRollbackRequest,
  TemplateDiffResponse,
  TemplateTestRequest,
  TemplateControlsResponse,
  UpdateControlsRequest,
  BulkCreateUsersRequest,
  SubscriberHashResponse,
  SystemStats,
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
    const headers = config.headers ?? {};
    const existingAuth = (headers as Record<string, string>).Authorization
      || (headers as Record<string, string>).authorization;
    const token = localStorage.getItem('access_token');
    if (!existingAuth && token) {
      (config.headers as Record<string, string>).Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Handle token refresh on 401 (with deduplication to prevent concurrent refresh calls)
let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

function onTokenRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token));
  refreshSubscribers = [];
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb);
}

api.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    // Don't attempt refresh for auth endpoints (prevents infinite loop)
    const isAuthEndpoint = originalRequest.url?.includes('/auth/refresh') ||
      originalRequest.url?.includes('/auth/login');

    if (error.response?.status === 401 && !originalRequest._retry && !isAuthEndpoint) {
      originalRequest._retry = true;

      if (isRefreshing) {
        // Another refresh is already in-flight — wait for it
        return new Promise((resolve) => {
          addRefreshSubscriber((newToken: string) => {
            originalRequest.headers.Authorization = `Bearer ${newToken}`;
            resolve(api(originalRequest));
          });
        });
      }

      isRefreshing = true;

      try {
        const refreshToken = localStorage.getItem('refresh_token');
        if (refreshToken) {
          const { data } = await api.post('/auth/refresh', {
            refresh_token: refreshToken,
          });

          localStorage.setItem('access_token', data.access_token);
          localStorage.setItem('refresh_token', data.refresh_token);

          originalRequest.headers.Authorization = `Bearer ${data.access_token}`;
          onTokenRefreshed(data.access_token);
          return api(originalRequest);
        }
      } catch (refreshError) {
        // Clear tokens and dispatch event for AuthProvider to handle
        localStorage.removeItem('access_token');
        localStorage.removeItem('refresh_token');
        refreshSubscribers = [];
        if (typeof window !== 'undefined') {
          window.dispatchEvent(new Event('auth:logout'));
        }
        return Promise.reject(refreshError);
      } finally {
        isRefreshing = false;
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
    return data.data?.applications ?? [];
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
    const { data } = await api.put<{ success: boolean; message: string }>(`/apps/${id}/settings`, settings);
    return data;
  },

  importResources: async (targetAppId: string, sourceAppId: string, resources: string[]) => {
    const { data } = await api.post<ApiResponse<{ linked: Record<string, number>; skipped: Record<string, number> }>>(`/apps/${targetAppId}/import`, {
      source_app_id: sourceAppId,
      resources,
    });
    return data.data;
  },

  listLinks: async (appId: string, resourceType?: string) => {
    const params = resourceType ? `?resource_type=${resourceType}` : '';
    const { data } = await api.get<ApiResponse<{ links: any[]; total_count: number }>>(`/apps/${appId}/links${params}`);
    return data.data;
  },

  removeLink: async (appId: string, linkId: string) => {
    await api.delete(`/apps/${appId}/links/${linkId}`);
  },

  removeAllLinks: async (appId: string) => {
    await api.delete(`/apps/${appId}/links`);
  },
};

// Helper to get auth headers — sends app API key via X-API-Key so the JWT
// in the Authorization header (added by the axios interceptor) is preserved.
// This allows the backend to identify both the app AND the dashboard user,
// enabling RBAC enforcement on API-key-protected routes.
const getAuthHeaders = (apiKey?: string) => {
  if (!apiKey) return {};
  return { 'X-API-Key': apiKey };
};

// ============= User APIs =============
interface UserListResponse {
  users: User[];
  total_count: number;
  page: number;
  page_size: number;
}

export const usersAPI = {
  list: async (apiKey: string, page = 1, pageSize = 20) => {
    const { data } = await api.get<ApiResponse<UserListResponse>>(`/users/?page=${page}&page_size=${pageSize}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
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
    const { data } = await api.post<{ success: boolean; message: string }>(`/users/${userId}/devices`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
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
    const { data } = await api.put<{ success: boolean; message: string }>(`/users/${userId}/preferences`, preferences, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getPreferences: async (apiKey: string, userId: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/users/${userId}/preferences`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  bulkCreate: async (apiKey: string, payload: BulkCreateUsersRequest) => {
    const { data } = await api.post('/users/bulk', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getSubscriberHash: async (apiKey: string, userId: string) => {
    const { data } = await api.get<SubscriberHashResponse>(`/users/${userId}/subscriber-hash`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
};

// ============= SSE Token APIs =============
export const sseAPI = {
  createToken: async (apiKey: string, userId: string): Promise<{ sse_token: string; user_id: string; expires_in: number }> => {
    const { data } = await api.post<{ sse_token: string; user_id: string; expires_in: number }>(
      '/sse/tokens',
      { user_id: userId },
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
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
  list: async (apiKey: string, page = 1, pageSize = 20, filters?: { status?: string; channel?: string; from?: string; to?: string; digest_key?: string }) => {
    const params = new URLSearchParams();
    params.set('page', String(page));
    params.set('page_size', String(pageSize));
    if (filters?.status && filters.status !== 'all') params.set('status', filters.status);
    if (filters?.channel && filters.channel !== 'all') params.set('channel', filters.channel);
    if (filters?.digest_key) params.set('digest_key', filters.digest_key);
    // Send date strings as-is (YYYY-MM-DD) so backend can parse and extend to_date to end of day
    if (filters?.from) params.set('from_date', filters.from);
    if (filters?.to) params.set('to_date', filters.to);

    // Note: This endpoint is currently NOT wrapped in success/data envelope in backend
    const { data } = await api.get<NotificationListResponse>(`/notifications/?${params.toString()}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
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

  // Batch
  sendBatch: async (apiKey: string, payload: BatchNotificationRequest) => {
    const { data } = await api.post('/notifications/batch', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  cancelBatch: async (apiKey: string, payload: CancelBatchRequest) => {
    await api.delete('/notifications/batch', {
      headers: getAuthHeaders(apiKey),
      data: payload,
    });
  },

  // Inbox operations
  getUnreadCount: async (apiKey: string, userId: string) => {
    const { data } = await api.get<UnreadCountResponse>(
      `/notifications/unread/count?user_id=${userId}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  listUnread: async (apiKey: string, userId: string, limit = 20, offset = 0) => {
    const { data } = await api.get<NotificationListResponse>(
      `/notifications/unread?user_id=${userId}&limit=${limit}&offset=${offset}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  markRead: async (apiKey: string, payload: MarkReadRequest) => {
    await api.post('/notifications/read', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  markAllRead: async (apiKey: string, payload: MarkAllReadRequest) => {
    await api.post('/notifications/read-all', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  bulkArchive: async (apiKey: string, payload: BulkArchiveRequest) => {
    await api.patch('/notifications/bulk/archive', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  snooze: async (apiKey: string, id: string, payload: SnoozeRequest) => {
    await api.post(`/notifications/${id}/snooze`, payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  unsnooze: async (apiKey: string, id: string) => {
    await api.post(`/notifications/${id}/unsnooze`, {}, {
      headers: getAuthHeaders(apiKey)
    });
  },
};

// ============= Quick-Send API =============
export const quickSendAPI = {
  send: async (apiKey: string, payload: QuickSendRequest) => {
    const { data } = await api.post<QuickSendResponse>('/quick-send', payload, {
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
  list: async (apiKey: string, limit = 20, offset = 0) => {
    // Note: This endpoint is currently NOT wrapped in success/data envelope in backend
    const { data } = await api.get<TemplateListResponse>(`/templates/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
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

  getLibrary: async (apiKey: string) => {
    const { data } = await api.get<{ templates: Template[] }>('/templates/library', {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  cloneFromLibrary: async (apiKey: string, name: string) => {
    const { data } = await api.post<Template>(`/templates/library/${name}/clone`, {}, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  rollback: async (apiKey: string, id: string, payload: TemplateRollbackRequest) => {
    const { data } = await api.post(`/templates/${id}/rollback`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  diff: async (apiKey: string, id: string, fromVersion: number, toVersion: number) => {
    const { data } = await api.get<TemplateDiffResponse>(
      `/templates/${id}/diff?from=${fromVersion}&to=${toVersion}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  sendTest: async (apiKey: string, id: string, payload: TemplateTestRequest) => {
    const { data } = await api.post(`/templates/${id}/test`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getControls: async (apiKey: string, id: string) => {
    const { data } = await api.get<TemplateControlsResponse>(`/templates/${id}/controls`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  updateControls: async (apiKey: string, id: string, payload: UpdateControlsRequest) => {
    const { data } = await api.put(`/templates/${id}/controls`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getVersion: async (apiKey: string, appId: string, templateName: string, version: number) => {
    const { data } = await api.get<TemplateVersion>(
      `/templates/${appId}/${templateName}/versions/${version}`,
      { headers: getAuthHeaders(apiKey) }
    );
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
    const { data } = await api.get<{ items: DLQItem[] }>('/admin/queues/dlq');
    return data.items;
  },

  replayDLQ: async (limit = 10) => {
    const { data } = await api.post<{ replayed_count: number }>(`/admin/queues/dlq/replay?limit=${limit}`);
    return data;
  },

  getProviderHealth: async () => {
    const { data } = await api.get<{ providers: Record<string, ProviderHealth> }>('/admin/providers/health');
    return data.providers;
  },

  createPlayground: async () => {
    const { data } = await api.post<{ id: string; url: string; expires_in: string }>('/admin/playground/webhook');
    return data;
  },

  getPlaygroundPayloads: async (id: string) => {
    const { data } = await api.get<{ id: string; payloads: any[]; count: number }>(`/playground/${id}`);
    return data;
  },

  // SSE Playground
  createSSEPlayground: async () => {
    const { data } = await api.post<{ id: string; sse_url: string; expires_in: string }>('/admin/playground/sse');
    return data;
  },

  sendSSETestMessage: async (id: string, payload?: { title?: string; body?: string; category?: string; data?: Record<string, unknown> }) => {
    const { data } = await api.post<{ status: string; user_id: string }>(`/admin/playground/sse/${id}/send`, payload || {});
    return data;
  },

  getAnalyticsSummary: async (period = '7d') => {
    const { data } = await api.get<AnalyticsSummary>(`/admin/analytics/summary?period=${period}`);
    return data;
  },

  // Dashboard notifications (org invites, etc.)
  listNotifications: async (limit = 50, offset = 0) => {
    const { data } = await api.get<{ notifications: DashboardNotification[]; total: number }>(
      `/admin/notifications?limit=${limit}&offset=${offset}`
    );
    return data;
  },

  getUnreadCount: async () => {
    const { data } = await api.get<{ unread_count: number }>('/admin/notifications/unread-count');
    return data.unread_count;
  },

  markNotificationsRead: async (ids: string[]) => {
    const { data } = await api.post<{ marked: number }>('/admin/notifications/read', { ids });
    return data.marked;
  },

  createDashboardSSEToken: async () => {
    const { data } = await api.post<{ sse_token: string; user_id: string; expires_in: number }>('/admin/sse/token');
    return data;
  },

  getSystemStats: async (): Promise<SystemStats> => {
    const [summary1d, summary7d] = await Promise.all([
      adminAPI.getAnalyticsSummary('1d'),
      adminAPI.getAnalyticsSummary('7d'),
    ]);
    return {
      total_apps: 0,
      total_users: summary7d.total_users ?? 0,
      total_templates: summary7d.total_templates ?? 0,
      total_workflows: summary7d.total_workflows ?? 0,
      notifications_today: summary1d.total_sent + summary1d.total_delivered + summary1d.total_read,
      notifications_this_week: summary7d.total_sent + summary7d.total_delivered + summary7d.total_read,
      success_rate: summary7d.success_rate,
    };
  },
};

// ============= Workflow APIs =============

export const workflowsAPI = {
  create: async (apiKey: string, payload: CreateWorkflowRequest) => {
    const { data } = await api.post<ApiResponse<Workflow>>('/workflows/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<ApiResponse<Workflow[]> & { total: number }>(`/workflows/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return { workflows: data.data, total: data.total };
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<Workflow>>(`/workflows/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  update: async (apiKey: string, id: string, payload: UpdateWorkflowRequest) => {
    const { data } = await api.put<ApiResponse<Workflow>>(`/workflows/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/workflows/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  trigger: async (apiKey: string, payload: TriggerWorkflowRequest) => {
    const { data } = await api.post<ApiResponse<WorkflowExecution>>('/workflows/trigger', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  triggerByTopic: async (apiKey: string, payload: TriggerByTopicRequest) => {
    const { data } = await api.post<ApiResponse<TriggerByTopicResult>>('/workflows/trigger-by-topic', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data as TriggerByTopicResult;
  },

  listExecutions: async (apiKey: string, limit = 20, offset = 0, workflowId?: string) => {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (workflowId) params.set('workflow_id', workflowId);
    const { data } = await api.get<ApiResponse<WorkflowExecution[]> & { total: number }>(`/workflows/executions?${params}`, {
      headers: getAuthHeaders(apiKey)
    });
    return { executions: data.data, total: data.total };
  },

  getExecution: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<WorkflowExecution>>(`/workflows/executions/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  cancelExecution: async (apiKey: string, id: string) => {
    await api.post(`/workflows/executions/${id}/cancel`, {}, {
      headers: getAuthHeaders(apiKey)
    });
  },

  // Phase 6: Schedules
  createSchedule: async (apiKey: string, payload: CreateScheduleRequest) => {
    const { data } = await api.post<ApiResponse<WorkflowSchedule>>('/workflows/schedules', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },
  listSchedules: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<ApiResponse<WorkflowSchedule[]> & { total: number }>(`/workflows/schedules?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return { schedules: data.data, total: data.total };
  },
  getSchedule: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<WorkflowSchedule>>(`/workflows/schedules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },
  updateSchedule: async (apiKey: string, id: string, payload: UpdateScheduleRequest) => {
    const { data } = await api.put<ApiResponse<WorkflowSchedule>>(`/workflows/schedules/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },
  deleteSchedule: async (apiKey: string, id: string) => {
    await api.delete(`/workflows/schedules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },
};

// ============= Digest Rule APIs =============

export const digestRulesAPI = {
  create: async (apiKey: string, payload: CreateDigestRuleRequest) => {
    const { data } = await api.post<ApiResponse<DigestRule>>('/digest-rules/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<ApiResponse<DigestRule[]> & { total: number }>(`/digest-rules/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return { rules: data.data, total: data.total };
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<DigestRule>>(`/digest-rules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  update: async (apiKey: string, id: string, payload: UpdateDigestRuleRequest) => {
    const { data } = await api.put<ApiResponse<DigestRule>>(`/digest-rules/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/digest-rules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },
};

// ============= Topic APIs =============

export const topicsAPI = {
  create: async (apiKey: string, payload: CreateTopicRequest) => {
    const { data } = await api.post<ApiResponse<Topic>>('/topics/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<ApiResponse<Topic[]> & { total: number }>(`/topics/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return { topics: data.data, total: data.total };
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<Topic>>(`/topics/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  getByKey: async (apiKey: string, key: string) => {
    const { data } = await api.get<ApiResponse<Topic>>(`/topics/key/${key}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  update: async (apiKey: string, id: string, payload: UpdateTopicRequest) => {
    const { data } = await api.put<ApiResponse<Topic>>(`/topics/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/topics/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  addSubscribers: async (apiKey: string, topicId: string, payload: TopicSubscribersRequest) => {
    const { data } = await api.post(`/topics/${topicId}/subscribers`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  removeSubscribers: async (apiKey: string, topicId: string, payload: TopicSubscribersRequest) => {
    await api.delete(`/topics/${topicId}/subscribers`, {
      headers: getAuthHeaders(apiKey),
      data: payload,
    });
  },

  getSubscribers: async (apiKey: string, topicId: string, limit = 20, offset = 0) => {
    const { data } = await api.get<ApiResponse<TopicSubscription[]> & { total: number }>(
      `/topics/${topicId}/subscribers?limit=${limit}&offset=${offset}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return { subscribers: data.data, total: data.total };
  },
};

// ============= Team / RBAC APIs =============
export const teamAPI = {
  inviteMember: async (appId: string, payload: InviteMemberRequest) => {
    const { data } = await api.post<AppMembership>(`/apps/${appId}/team/`, payload);
    return data;
  },

  listMembers: async (appId: string) => {
    const { data } = await api.get<{ members: AppMembership[] }>(`/apps/${appId}/team/`);
    return data.members;
  },

  updateRole: async (appId: string, membershipId: string, payload: UpdateRoleRequest) => {
    const { data } = await api.put<AppMembership>(`/apps/${appId}/team/${membershipId}`, payload);
    return data;
  },

  removeMember: async (appId: string, membershipId: string) => {
    await api.delete(`/apps/${appId}/team/${membershipId}`);
  },
};

// ============= Audit Log APIs =============
// Backend returns { audit_logs: [...], count: N }
interface AuditLogListResponse {
  audit_logs: AuditLog[];
  count: number;
}

export const auditAPI = {
  list: async (filters?: AuditLogFilters) => {
    const params = new URLSearchParams();
    if (filters?.app_id) params.set('app_id', filters.app_id);
    if (filters?.actor_id) params.set('actor_id', filters.actor_id);
    if (filters?.action) params.set('action', filters.action);
    if (filters?.resource) params.set('resource', filters.resource);
    if (filters?.from_date) params.set('from_date', filters.from_date);
    if (filters?.to_date) params.set('to_date', filters.to_date);
    if (filters?.limit) params.set('limit', String(filters.limit));
    if (filters?.offset) params.set('offset', String(filters.offset));

    const { data } = await api.get<AuditLogListResponse>(`/admin/audit/?${params}`);
    return data;
  },

  get: async (id: string) => {
    const { data } = await api.get<AuditLog>(`/admin/audit/${id}`);
    return data;
  },
};

// ============= Environment APIs =============
export const environmentsAPI = {
  create: async (appId: string, payload: CreateEnvironmentRequest) => {
    const { data } = await api.post<ApiResponse<Environment>>(`/apps/${appId}/environments`, payload);
    return data.data;
  },

  list: async (appId: string) => {
    const { data } = await api.get<ApiResponse<Environment[]>>(`/apps/${appId}/environments`);
    return data.data;
  },

  get: async (appId: string, envId: string) => {
    const { data } = await api.get<ApiResponse<Environment>>(`/apps/${appId}/environments/${envId}`);
    return data.data;
  },

  delete: async (appId: string, envId: string) => {
    await api.delete(`/apps/${appId}/environments/${envId}`);
  },

  promote: async (appId: string, payload: PromoteEnvironmentRequest) => {
    const { data } = await api.post<ApiResponse<any>>(`/apps/${appId}/environments/promote`, payload);
    return data.data;
  },
};

// ============= Tenant APIs (C1) =============
export const tenantsAPI = {
  create: async (payload: CreateTenantRequest) => {
    const { data } = await api.post<ApiResponse<Tenant>>('/tenants', payload);
    return data.data;
  },

  list: async () => {
    const { data } = await api.get<ApiResponse<Tenant[]>>('/tenants');
    return data.data ?? [];
  },

  get: async (id: string) => {
    const { data } = await api.get<ApiResponse<Tenant>>(`/tenants/${id}`);
    return data.data;
  },

  update: async (id: string, payload: { name?: string }) => {
    const { data } = await api.put<ApiResponse<Tenant>>(`/tenants/${id}`, payload);
    return data.data;
  },

  delete: async (id: string) => {
    await api.delete(`/tenants/${id}`);
  },

  listMembers: async (id: string) => {
    const { data } = await api.get<ApiResponse<TenantMember[]>>(`/tenants/${id}/members`);
    return data.data ?? [];
  },

  inviteMember: async (id: string, payload: InviteTenantMemberRequest) => {
    const { data } = await api.post<ApiResponse<TenantMember>>(`/tenants/${id}/members`, payload);
    return data.data;
  },

  updateMemberRole: async (id: string, memberId: string, role: 'owner' | 'admin' | 'member') => {
    const { data } = await api.put<ApiResponse<TenantMember>>(`/tenants/${id}/members/${memberId}`, { role });
    return data.data;
  },

  removeMember: async (id: string, memberId: string) => {
    await api.delete(`/tenants/${id}/members/${memberId}`);
  },

  getBilling: async (id: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/tenants/${id}/billing`);
    return data.data;
  },

  checkoutBilling: async (id: string, tier: string = 'pro') => {
    const { data } = await api.post<ApiResponse<any>>(`/tenants/${id}/billing/checkout`, { tier });
    return data;
  }
};

// ============= Billing APIs (user-facing, JWT auth) =============
export const billingAPI = {
  getUsage: async () => {
    const { data } = await api.get('/billing/usage');
    return data;
  },

  getSubscription: async () => {
    const { data } = await api.get('/billing/subscription');
    return data;
  },

  acceptTrial: async () => {
    const { data } = await api.post('/billing/accept-trial');
    return data;
  },

  getUsageBreakdown: async () => {
    const { data } = await api.get('/billing/usage/breakdown');
    return data;
  },

  getRates: async () => {
    const { data } = await api.get('/billing/rates');
    return data;
  },
};

// ============= Custom Provider APIs =============
export const providersAPI = {
  register: async (appId: string, payload: RegisterProviderRequest) => {
    const { data } = await api.post<ApiResponse<CustomProvider>>(`/apps/${appId}/providers`, payload);
    return data.data;
  },

  list: async (appId: string) => {
    const { data } = await api.get<ApiResponse<CustomProvider[]>>(`/apps/${appId}/providers`);
    return data.data;
  },

  remove: async (appId: string, providerId: string) => {
    await api.delete(`/apps/${appId}/providers/${providerId}`);
  },
};

// ============= Presence API =============
export const presenceAPI = {
  checkIn: async (apiKey: string, payload: PresenceCheckInRequest) => {
    const { data } = await api.post('/presence/check-in', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
};

// ============= Auth Extended APIs =============
export const authExtendedAPI = {
  changePassword: async (payload: { old_password: string; new_password: string }) => {
    await api.post('/admin/change-password', payload);
  },

  deleteOwnAccount: async (payload: { password: string; confirm_text: string }) => {
    await api.delete('/admin/me', { data: payload });
  },

  sendPhoneOTP: async (payload: { phone: string }) => {
    await api.post('/admin/phone/send-otp', payload);
  },

  verifyPhoneOTP: async (payload: { phone: string; otp_code: string }) => {
    await api.post('/admin/phone/verify-otp', payload);
  },
};

// ============= Media Upload APIs =============
export const mediaAPI = {
  upload: async (apiKey: string, file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    const { data } = await api.post<{ url: string; filename: string; content_type: string; size: number }>('/media/upload', formData, {
      headers: {
        ...getAuthHeaders(apiKey),
        'Content-Type': 'multipart/form-data',
      },
    });
    return data;
  },
};

export default api;