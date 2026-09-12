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
  RenderTemplateResponse,
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
  ProviderTestResponse,
  ProviderRotateResponse,
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
  BulkCreateUserRequest,
  BulkCreateUserResponse,
  SubscriberHashResponse,
  SystemStats,
  BillingUsage,
  BillingSubscription,
  BillingUsageBreakdown,
  BillingRates,
  AcceptTrialResponse,
  BillingPlanBundle,
  PublicBillingPricing,
  BizProduct,
  BizPlan,
  BizPlanAddon,
  BizConfig,
  BizSubscription,
  BizInvoice,
  BizPayment,
  CreateBizSubscriptionRequest,
  UpdateBizSubscriptionRequest,
  CreateBizInvoiceRequest,
  UpdateBizInvoiceRequest,
  RecordPaymentRequest,
  ChangeBizPlanRequest,
  RefundBizPaymentRequest,
  ApplyBizCouponRequest,
  ApplyBizCreditRequest,
  BizEstimate,
  CreateBizEstimateRequest,
  BizCoupon,
  CreateBizCouponRequest,
  BulkGenerateBizCouponsRequest,
  BizCreditNote,
  CreateBizCreditNoteRequest,
  BizRetainer,
  CreateBizRetainerRequest,
  BizContract,
  CreateBizContractRequest,
  AmendBizContractRequest,
  BizUsageMeter,
  CreateBizUsageMeterRequest,
  ReportBizUsageRequest,
  BizUsageSummary,
  BizExpense,
  CreateBizExpenseRequest,
  BizConnector,
  CreateBizConnectorRequest,
  UpdateBizConnectorRequest,
  BizStatement,
  BizPortalLink,
  BizListResponse,
} from '../types';

// Resolve API base URL:
// - Vite dev (npm run dev, Docker UI): always use relative /v1 so the dev-server proxy
//   (API_PROXY_TARGET → notification-service:8080 in Docker) is used. A baked
//   VITE_API_BASE_URL in ui/.env would otherwise send the browser to the wrong host
//   (e.g. localhost:8080 on the client) and hang /auth and ProtectedRoute.
// - Production builds: use VITE_API_BASE_URL when set (e.g. Vercel), else same-origin /v1.
const rawApiBaseUrl = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim() || '';
const normalizedApiBaseUrl = rawApiBaseUrl.replace(/\/+$/, '').replace(/\/v1$/i, '');
const isLocalUiHost =
  typeof window !== 'undefined' &&
  (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1');
const pointsToLoopbackApi = /^https?:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/i.test(normalizedApiBaseUrl);
export const API_V1_BASE_URL = import.meta.env.DEV
  ? '/v1'
  : normalizedApiBaseUrl && !(isLocalUiHost && pointsToLoopbackApi)
    ? `${normalizedApiBaseUrl}/v1`
    : '/v1';

export const buildApiUrl = (path: string) => {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`;
  return `${API_V1_BASE_URL}${normalizedPath}`;
};

const api = axios.create({
  baseURL: API_V1_BASE_URL,
  timeout: 30_000,
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

  getCodeSamples: async (id: string, language?: string) => {
    const params = language ? `?language=${language}` : '';
    const { data } = await api.get<ApiResponse<Record<string, Record<string, any>>>>(`/apps/${id}/code-samples${params}`);
    return data.data;
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
  list: async (apiKey: string, page = 1, pageSize = 20, search?: string) => {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    const trimmed = search?.trim();
    if (trimmed) params.set('search', trimmed);
    const { data } = await api.get<ApiResponse<UserListResponse>>(`/users/?${params.toString()}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data.data;
  },

  /** Fetches every page for the given optional search filter (page_size capped at 100). */
  listAll: async (apiKey: string, search?: string) => {
    const pageSize = 100;
    let page = 1;
    const users: User[] = [];
    let total_count = 0;

    while (true) {
      const res = await usersAPI.list(apiKey, page, pageSize, search);
      const batch = res.users ?? [];
      users.push(...batch);
      total_count = res.total_count ?? users.length;
      if (users.length >= total_count || batch.length === 0) break;
      page += 1;
    }

    return { users, total_count };
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

  bulkCreate: async (apiKey: string, payload: BulkCreateUserRequest) => {
    const { data } = await api.post<BulkCreateUserResponse>('/users/bulk', payload, {
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

  renderLibrary: async (apiKey: string, name: string, payload: RenderTemplateRequest) => {
    const { data } = await api.post<RenderTemplateResponse>(`/templates/library/${name}/render`, payload, {
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
  }
};

// ============= Billing APIs (user-facing, JWT auth) =============
export const billingAPI = {
  getUsage: async () => {
    const { data } = await api.get<BillingUsage>('/billing/usage');
    return data;
  },

  getSubscription: async () => {
    const { data } = await api.get<BillingSubscription>('/billing/subscription');
    return data;
  },

  acceptTrial: async () => {
    const { data } = await api.post<AcceptTrialResponse>('/billing/accept-trial');
    return data;
  },

  getUsageBreakdown: async (appId?: string) => {
    const { data } = await api.get<BillingUsageBreakdown>('/billing/usage/breakdown', {
      params: appId ? { app_id: appId } : undefined,
    });
    return data;
  },

  getRates: async () => {
    const { data } = await api.get<BillingRates>('/billing/rates');
    return data;
  },

  getPlans: async () => {
    const { data } = await api.get<{ currency: string; active_version: string; plans: BillingPlanBundle[] }>('/billing/plans');
    return data;
  },

  checkoutBilling: async (planId: string) => {
    const { data } = await api.post<any>('/billing/checkout', { tier: planId });
    return data;
  },

  verifyPayment: async (payload: { razorpay_order_id: string, razorpay_payment_id: string, razorpay_signature: string }) => {
    const { data } = await api.post<any>('/billing/verify-payment', payload);
    return data;
  }
};

// ============= Public Billing API (no auth) =============
export const publicBillingAPI = {
  getPricing: async () => {
    const { data } = await api.get<PublicBillingPricing>('/public/billing/pricing');
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

  test: async (appId: string, providerId: string) => {
    const { data } = await api.post<ApiResponse<ProviderTestResponse>>(`/apps/${appId}/providers/${providerId}/test`);
    return data.data;
  },

  rotate: async (appId: string, providerId: string) => {
    const { data } = await api.post<ApiResponse<ProviderRotateResponse>>(`/apps/${appId}/providers/${providerId}/rotate`);
    return data.data;
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

// ============= Twilio Content Template APIs (API-key-protected) =============
export const twilioTemplatesAPI = {
  list: async (apiKey: string, search?: string) => {
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    const qs = params.toString() ? `?${params}` : '';
    const { data } = await api.get<ApiResponse<any>>(`/twilio/templates/${qs}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  get: async (apiKey: string, contentSid: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/twilio/templates/${contentSid}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  create: async (apiKey: string, payload: { friendly_name: string; language: string; types: Record<string, any>; variables?: Record<string, string> }) => {
    const { data } = await api.post<ApiResponse<any>>('/twilio/templates/', payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  update: async (apiKey: string, contentSid: string, payload: Record<string, any>) => {
    const { data } = await api.put<ApiResponse<any>>(`/twilio/templates/${contentSid}`, payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  delete: async (apiKey: string, contentSid: string) => {
    await api.delete(`/twilio/templates/${contentSid}`, {
      headers: getAuthHeaders(apiKey),
    });
  },

  submitApproval: async (apiKey: string, contentSid: string, payload: { name: string; category: string }) => {
    const { data } = await api.post<ApiResponse<any>>(`/twilio/templates/${contentSid}/approve`, payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  getApprovalStatus: async (apiKey: string, contentSid: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/twilio/templates/${contentSid}/approval`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  sync: async (apiKey: string, contentSid: string) => {
    const { data } = await api.post<ApiResponse<any>>(`/twilio/templates/${contentSid}/sync`, {}, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  preview: async (apiKey: string, contentSid: string, variables: Record<string, string>) => {
    const { data } = await api.post<ApiResponse<any>>(`/twilio/templates/${contentSid}/preview`, { variables }, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },
};

// ============= WhatsApp Admin APIs (JWT-protected) =============
export const whatsappAdminAPI = {
  getStatus: async (appId: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/admin/whatsapp/${appId}/status`);
    return data.data;
  },

  connect: async (payload: { code: string; app_id: string }) => {
    const { data } = await api.post<ApiResponse<any>>('/admin/whatsapp/connect', payload);
    return data.data;
  },

  // manualConnect skips Embedded Signup and stores a pre-existing System User
  // access token directly. Use this while the Meta App is still in Development
  // mode / pending App Review, or for self-hosted single-tenant deployments.
  manualConnect: async (payload: {
    app_id: string;
    access_token: string;
    waba_id: string;
    phone_number_id?: string;
  }) => {
    const { data } = await api.post<ApiResponse<any>>('/admin/whatsapp/manual-connect', payload);
    return data.data;
  },

  disconnect: async (appId: string) => {
    const { data } = await api.post<{ success: boolean; message: string }>(`/admin/whatsapp/${appId}/disconnect`);
    return data;
  },

  subscribeWebhooks: async (appId: string) => {
    const { data } = await api.post<{ success: boolean; message: string }>(`/admin/whatsapp/${appId}/subscribe-webhooks`);
    return data;
  },
};

// ============= WhatsApp Template APIs (API-key-protected) =============
export const whatsappTemplatesAPI = {
  create: async (apiKey: string, payload: Record<string, any>) => {
    const { data } = await api.post<ApiResponse<any>>('/whatsapp/templates/', payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  list: async (apiKey: string, filters?: { name?: string; status?: string }) => {
    const params = new URLSearchParams();
    if (filters?.name) params.set('name', filters.name);
    if (filters?.status) params.set('status', filters.status);
    const { data } = await api.get<ApiResponse<any>>(`/whatsapp/templates/?${params}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  get: async (apiKey: string, name: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/whatsapp/templates/${name}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  delete: async (apiKey: string, name: string) => {
    await api.delete(`/whatsapp/templates/${name}`, {
      headers: getAuthHeaders(apiKey),
    });
  },

  sync: async (apiKey: string, name: string) => {
    const { data } = await api.post<ApiResponse<any>>(`/whatsapp/templates/${name}/sync`, {}, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },
};

// ============= WhatsApp Rich-Template APIs (Phase 1-3 of rich plan) =============
//
// Authored carousels / coupons / cta_url / quick_reply / list templates that
// FRN persists and submits to Meta (and Twilio in parallel when configured).
// All endpoints are API-key protected and scoped by app via the auth header.
export const whatsappRichTemplatesAPI = {
  create: async (apiKey: string, payload: Record<string, any>) => {
    const { data } = await api.post<ApiResponse<any>>('/whatsapp/rich-templates/', payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  list: async (
    apiKey: string,
    filters?: { kind?: string; status?: string; name_prefix?: string; limit?: number; offset?: number },
  ) => {
    const params = new URLSearchParams();
    if (filters?.kind) params.set('kind', filters.kind);
    if (filters?.status) params.set('status', filters.status);
    if (filters?.name_prefix) params.set('name_prefix', filters.name_prefix);
    if (filters?.limit != null) params.set('limit', String(filters.limit));
    if (filters?.offset != null) params.set('offset', String(filters.offset));
    const { data } = await api.get<ApiResponse<any[]> & { total: number }>(`/whatsapp/rich-templates/?${params}`, {
      headers: getAuthHeaders(apiKey),
    });
    return { templates: data.data || [], total: data.total };
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<ApiResponse<any>>(`/whatsapp/rich-templates/${id}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/whatsapp/rich-templates/${id}`, {
      headers: getAuthHeaders(apiKey),
    });
  },

  sync: async (apiKey: string, id: string) => {
    const { data } = await api.post<ApiResponse<any>>(`/whatsapp/rich-templates/${id}/sync`, {}, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },

  preview: async (apiKey: string, id: string, variables?: Record<string, string>) => {
    const { data } = await api.post<ApiResponse<any>>(`/whatsapp/rich-templates/${id}/preview`, { variables }, {
      headers: getAuthHeaders(apiKey),
    });
    return data.data;
  },
};

// ============= WhatsApp Conversation APIs (API-key-protected) =============
export const whatsappConversationsAPI = {
  list: async (apiKey: string, limit = 50, offset = 0) => {
    const { data } = await api.get<ApiResponse<any[]> & { total: number }>(`/whatsapp/conversations/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey),
    });
    return { conversations: data.data, total: data.total };
  },

  getMessages: async (apiKey: string, contactId: string, limit = 50, offset = 0) => {
    const { data } = await api.get<ApiResponse<any[]> & { total: number }>(`/whatsapp/conversations/${contactId}/messages?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey),
    });
    return { messages: data.data, total: data.total };
  },

  reply: async (apiKey: string, contactId: string, payload: { text?: string; template_name?: string }) => {
    const { data } = await api.post<{ success: boolean; message: string }>(`/whatsapp/conversations/${contactId}/reply`, payload, {
      headers: getAuthHeaders(apiKey),
    });
    return data;
  },

  markRead: async (apiKey: string, contactId: string) => {
    const { data } = await api.post<{ success: boolean; message: string }>(`/whatsapp/conversations/${contactId}/read`, {}, {
      headers: getAuthHeaders(apiKey),
    });
    return data;
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

// ============= Files API (managed file store) =============
//
// Backs the `file_id` attachment source. Files are tenant-scoped; cross-tenant
// access returns 404 by design. See documents/FILE_ATTACHMENTS_GUIDE.md.
export const filesAPI = {
  /** Multipart upload. `onProgress` (0..1) is invoked while the body uploads. */
  upload: async (
    apiKey: string,
    file: File,
    onProgress?: (progress: number) => void,
  ): Promise<import('../types').FileObject> => {
    const formData = new FormData();
    formData.append('file', file);
    const { data } = await api.post<import('../types').FileObject>('/files', formData, {
      headers: {
        ...getAuthHeaders(apiKey),
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (e) => {
        if (onProgress && e.total) onProgress(e.loaded / e.total);
      },
    });
    return data;
  },

  list: async (apiKey: string, opts: import('../types').ListFilesOptions = {}) => {
    const params = new URLSearchParams();
    if (opts.limit !== undefined) params.set('limit', String(opts.limit));
    if (opts.offset !== undefined) params.set('offset', String(opts.offset));
    const qs = params.toString();
    const { data } = await api.get<import('../types').FileListResponse>(
      `/files${qs ? `?${qs}` : ''}`,
      { headers: getAuthHeaders(apiKey) },
    );
    return data;
  },

  get: async (apiKey: string, fileId: string): Promise<import('../types').FileObject> => {
    const { data } = await api.get<import('../types').FileObject>(`/files/${encodeURIComponent(fileId)}`, {
      headers: getAuthHeaders(apiKey),
    });
    return data;
  },

  delete: async (apiKey: string, fileId: string) => {
    await api.delete(`/files/${encodeURIComponent(fileId)}`, {
      headers: getAuthHeaders(apiKey),
    });
  },

  /** Mint a short-lived public signed download URL (default 15 min TTL). */
  signedDownloadUrl: async (apiKey: string, fileId: string): Promise<import('../types').FileSignedURL> => {
    const { data } = await api.get<import('../types').FileSignedURL>(
      `/files/${encodeURIComponent(fileId)}/download-url`,
      { headers: getAuthHeaders(apiKey) },
    );
    return data;
  },

  /** Authenticated streaming content fetch (returns a Blob). */
  content: async (apiKey: string, fileId: string): Promise<Blob> => {
    const { data } = await api.get<Blob>(`/files/${encodeURIComponent(fileId)}/content`, {
      headers: getAuthHeaders(apiKey),
      responseType: 'blob',
    });
    return data;
  },
};

// ============= Business Billing API =============
//
// Dashboard calls send X-API-Key (app) + JWT (user) via getAuthHeaders +
// the axios interceptor. Responses are { success, data [, total] }.
type BizOne<T> = { success: boolean; data: T };

const bizOpts = (apiKey: string) => ({ headers: getAuthHeaders(apiKey) });

export const bizBillingAPI = {
  // ── Products ──
  listProducts: async (apiKey: string) => {
    const { data } = await api.get<BizOne<BizProduct[]>>('/biz/products', bizOpts(apiKey));
    return data.data;
  },
  getProduct: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizProduct>>(`/biz/products/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createProduct: async (apiKey: string, payload: Partial<BizProduct>) => {
    const { data } = await api.post<BizOne<BizProduct>>('/biz/products', payload, bizOpts(apiKey));
    return data.data;
  },
  updateProduct: async (apiKey: string, id: string, payload: Partial<BizProduct>) => {
    const { data } = await api.put<BizOne<BizProduct>>(`/biz/products/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteProduct: async (apiKey: string, id: string) => {
    await api.delete(`/biz/products/${id}`, bizOpts(apiKey));
  },

  // ── Plans & Addons ──
  listPlans: async (apiKey: string) => {
    const { data } = await api.get<BizOne<BizPlan[]>>('/biz/plans', bizOpts(apiKey));
    return data.data;
  },
  getPlan: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizPlan>>(`/biz/plans/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createPlan: async (apiKey: string, payload: Partial<BizPlan>) => {
    const { data } = await api.post<BizOne<BizPlan>>('/biz/plans', payload, bizOpts(apiKey));
    return data.data;
  },
  updatePlan: async (apiKey: string, id: string, payload: Partial<BizPlan>) => {
    const { data } = await api.put<BizOne<BizPlan>>(`/biz/plans/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deletePlan: async (apiKey: string, id: string) => {
    await api.delete(`/biz/plans/${id}`, bizOpts(apiKey));
  },
  listPlanAddons: async (apiKey: string, planId: string) => {
    const { data } = await api.get<BizOne<BizPlanAddon[]>>(`/biz/plans/${planId}/addons`, bizOpts(apiKey));
    return data.data;
  },
  addPlanAddon: async (apiKey: string, planId: string, payload: Partial<BizPlanAddon>) => {
    const { data } = await api.post<BizOne<BizPlanAddon>>(`/biz/plans/${planId}/addons`, payload, bizOpts(apiKey));
    return data.data;
  },
  removePlanAddon: async (apiKey: string, planId: string, addonId: string) => {
    await api.delete(`/biz/plans/${planId}/addons/${addonId}`, bizOpts(apiKey));
  },

  // ── Config ──
  getConfig: async (apiKey: string, configType: string) => {
    const { data } = await api.get<BizOne<BizConfig>>(`/biz/config/${configType}`, bizOpts(apiKey));
    return data.data;
  },
  updateConfig: async (apiKey: string, configType: string, payload: Partial<BizConfig>) => {
    const { data } = await api.put<BizOne<BizConfig>>(`/biz/config/${configType}`, payload, bizOpts(apiKey));
    return data.data;
  },

  // ── Subscriptions ──
  listSubscriptions: async (apiKey: string, params?: { user_id?: string; plan_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizSubscription>>('/biz/subscriptions', { ...bizOpts(apiKey), params });
    return data;
  },
  getSubscription: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizSubscription>>(`/biz/subscriptions/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createSubscription: async (apiKey: string, payload: CreateBizSubscriptionRequest) => {
    const { data } = await api.post<BizOne<BizSubscription>>('/biz/subscriptions', payload, bizOpts(apiKey));
    return data.data;
  },
  updateSubscription: async (apiKey: string, id: string, payload: UpdateBizSubscriptionRequest) => {
    const { data } = await api.put<BizOne<BizSubscription>>(`/biz/subscriptions/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  changePlan: async (apiKey: string, id: string, payload: ChangeBizPlanRequest) => {
    const { data } = await api.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/change-plan`, payload, bizOpts(apiKey));
    return data.data;
  },
  cancelSubscription: async (apiKey: string, id: string, atPeriodEnd = false) => {
    const { data } = await api.post<BizOne<BizSubscription>>(
      `/biz/subscriptions/${id}/cancel`,
      {},
      { ...bizOpts(apiKey), params: { at_period_end: atPeriodEnd } },
    );
    return data.data;
  },
  pauseSubscription: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/pause`, {}, bizOpts(apiKey));
    return data.data;
  },
  resumeSubscription: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/resume`, {}, bizOpts(apiKey));
    return data.data;
  },
  reactivateSubscription: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/reactivate`, {}, bizOpts(apiKey));
    return data.data;
  },
  setNonRenewing: async (apiKey: string, id: string, nonRenewing: boolean) => {
    const { data } = await api.post<BizOne<BizSubscription>>(
      `/biz/subscriptions/${id}/non-renewing`,
      { non_renewing: nonRenewing },
      bizOpts(apiKey),
    );
    return data.data;
  },

  // ── Invoices ──
  listInvoices: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizInvoice>>('/biz/invoices', { ...bizOpts(apiKey), params });
    return data;
  },
  getInvoice: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizInvoice>>(`/biz/invoices/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createInvoice: async (apiKey: string, payload: CreateBizInvoiceRequest) => {
    const { data } = await api.post<BizOne<BizInvoice>>('/biz/invoices', payload, bizOpts(apiKey));
    return data.data;
  },
  updateDraftInvoice: async (apiKey: string, id: string, payload: UpdateBizInvoiceRequest) => {
    const { data } = await api.put<BizOne<BizInvoice>>(`/biz/invoices/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  issueInvoice: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/issue`, {}, bizOpts(apiKey));
    return data.data;
  },
  sendInvoice: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  voidInvoice: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/void`, {}, bizOpts(apiKey));
    return data.data;
  },
  writeOffInvoice: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/write-off`, {}, bizOpts(apiKey));
    return data.data;
  },
  applyCoupon: async (apiKey: string, id: string, payload: ApplyBizCouponRequest) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/apply-coupon`, payload, bizOpts(apiKey));
    return data.data;
  },
  applyCredit: async (apiKey: string, id: string, payload: ApplyBizCreditRequest = {}) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/apply-credit`, payload, bizOpts(apiKey));
    return data.data;
  },
  addLateFee: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/late-fee`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Payments ──
  recordPayment: async (apiKey: string, invoiceId: string, payload: RecordPaymentRequest) => {
    const { data } = await api.post<BizOne<BizPayment>>(`/biz/invoices/${invoiceId}/payments`, payload, bizOpts(apiKey));
    return data.data;
  },
  listPayments: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizPayment>>('/biz/payments', { ...bizOpts(apiKey), params });
    return data;
  },
  getPayment: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizPayment>>(`/biz/payments/${id}`, bizOpts(apiKey));
    return data.data;
  },
  refundPayment: async (apiKey: string, id: string, payload: RefundBizPaymentRequest = {}) => {
    const { data } = await api.post<BizOne<BizPayment>>(`/biz/payments/${id}/refund`, payload, bizOpts(apiKey));
    return data.data;
  },

  // ── Estimates ──
  listEstimates: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizEstimate>>('/biz/estimates', { ...bizOpts(apiKey), params });
    return data;
  },
  getEstimate: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizEstimate>>(`/biz/estimates/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createEstimate: async (apiKey: string, payload: CreateBizEstimateRequest) => {
    const { data } = await api.post<BizOne<BizEstimate>>('/biz/estimates', payload, bizOpts(apiKey));
    return data.data;
  },
  updateEstimate: async (apiKey: string, id: string, payload: Partial<CreateBizEstimateRequest>) => {
    const { data } = await api.put<BizOne<BizEstimate>>(`/biz/estimates/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteEstimate: async (apiKey: string, id: string) => {
    await api.delete(`/biz/estimates/${id}`, bizOpts(apiKey));
  },
  sendEstimate: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  acceptEstimate: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/accept`, {}, bizOpts(apiKey));
    return data.data;
  },
  rejectEstimate: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/reject`, {}, bizOpts(apiKey));
    return data.data;
  },
  convertEstimate: async (apiKey: string, id: string, lineIndexes?: number[]) => {
    const { data } = await api.post<BizOne<BizInvoice>>(
      `/biz/estimates/${id}/convert`,
      lineIndexes?.length ? { line_indexes: lineIndexes } : {},
      bizOpts(apiKey),
    );
    return data.data;
  },

  // ── Coupons ──
  listCoupons: async (apiKey: string, params?: { code?: string; active?: boolean }) => {
    const { data } = await api.get<BizListResponse<BizCoupon>>('/biz/coupons', { ...bizOpts(apiKey), params });
    return data;
  },
  getCoupon: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizCoupon>>(`/biz/coupons/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createCoupon: async (apiKey: string, payload: CreateBizCouponRequest) => {
    const { data } = await api.post<BizOne<BizCoupon>>('/biz/coupons', payload, bizOpts(apiKey));
    return data.data;
  },
  bulkGenerateCoupons: async (apiKey: string, payload: BulkGenerateBizCouponsRequest) => {
    const { data } = await api.post<BizOne<BizCoupon[]>>('/biz/coupons/bulk-generate', payload, bizOpts(apiKey));
    return data.data;
  },
  updateCoupon: async (apiKey: string, id: string, payload: Partial<CreateBizCouponRequest> & { active?: boolean }) => {
    const { data } = await api.put<BizOne<BizCoupon>>(`/biz/coupons/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteCoupon: async (apiKey: string, id: string) => {
    await api.delete(`/biz/coupons/${id}`, bizOpts(apiKey));
  },

  // ── Credit notes & retainers ──
  listCreditNotes: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizCreditNote>>('/biz/credit-notes', { ...bizOpts(apiKey), params });
    return data;
  },
  getCreditNote: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizCreditNote>>(`/biz/credit-notes/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createCreditNote: async (apiKey: string, payload: CreateBizCreditNoteRequest) => {
    const { data } = await api.post<BizOne<BizCreditNote>>('/biz/credit-notes', payload, bizOpts(apiKey));
    return data.data;
  },
  refundCreditNote: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizCreditNote>>(`/biz/credit-notes/${id}/refund`, {}, bizOpts(apiKey));
    return data.data;
  },
  listRetainers: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizRetainer>>('/biz/retainers', { ...bizOpts(apiKey), params });
    return data;
  },
  getRetainer: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizRetainer>>(`/biz/retainers/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createRetainer: async (apiKey: string, payload: CreateBizRetainerRequest) => {
    const { data } = await api.post<BizOne<BizRetainer>>('/biz/retainers', payload, bizOpts(apiKey));
    return data.data;
  },
  markRetainerPaid: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizRetainer>>(`/biz/retainers/${id}/mark-paid`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Contracts ──
  listContracts: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await api.get<BizListResponse<BizContract>>('/biz/contracts', { ...bizOpts(apiKey), params });
    return data;
  },
  getContract: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizContract>>(`/biz/contracts/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createContract: async (apiKey: string, payload: CreateBizContractRequest) => {
    const { data } = await api.post<BizOne<BizContract>>('/biz/contracts', payload, bizOpts(apiKey));
    return data.data;
  },
  updateContract: async (apiKey: string, id: string, payload: Partial<CreateBizContractRequest>) => {
    const { data } = await api.put<BizOne<BizContract>>(`/biz/contracts/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  sendContract: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizContract>>(`/biz/contracts/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  acceptContract: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizContract>>(`/biz/contracts/${id}/accept`, {}, bizOpts(apiKey));
    return data.data;
  },
  amendContract: async (apiKey: string, id: string, payload: AmendBizContractRequest) => {
    const { data } = await api.post<BizOne<BizContract>>(`/biz/contracts/${id}/amend`, payload, bizOpts(apiKey));
    return data.data;
  },
  renewContract: async (apiKey: string, id: string, newEndDate?: string) => {
    const { data } = await api.post<BizOne<BizContract>>(
      `/biz/contracts/${id}/renew`,
      newEndDate ? { new_end_date: newEndDate } : {},
      bizOpts(apiKey),
    );
    return data.data;
  },
  terminateContract: async (apiKey: string, id: string) => {
    const { data } = await api.post<BizOne<BizContract>>(`/biz/contracts/${id}/terminate`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Usage ──
  listUsageMeters: async (apiKey: string) => {
    const { data } = await api.get<BizOne<BizUsageMeter[]>>('/biz/usage/meters', bizOpts(apiKey));
    return data.data;
  },
  createUsageMeter: async (apiKey: string, payload: CreateBizUsageMeterRequest) => {
    const { data } = await api.post<BizOne<BizUsageMeter>>('/biz/usage/meters', payload, bizOpts(apiKey));
    return data.data;
  },
  reportUsage: async (apiKey: string, payload: ReportBizUsageRequest) => {
    const { data } = await api.post<{ success: boolean; accepted: number }>('/biz/usage/events', payload, bizOpts(apiKey));
    return data.accepted;
  },
  getUsageSummary: async (
    apiKey: string,
    params: { user_id: string; subscription_id?: string; from?: string; to?: string },
  ) => {
    const { data } = await api.get<BizOne<BizUsageSummary[]>>('/biz/usage/summary', { ...bizOpts(apiKey), params });
    return data.data;
  },

  // ── Expenses ──
  listExpenses: async (apiKey: string, params?: { category?: string; user_id?: string; billable?: boolean }) => {
    const { data } = await api.get<BizListResponse<BizExpense>>('/biz/expenses', { ...bizOpts(apiKey), params });
    return data;
  },
  getExpense: async (apiKey: string, id: string) => {
    const { data } = await api.get<BizOne<BizExpense>>(`/biz/expenses/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createExpense: async (apiKey: string, payload: CreateBizExpenseRequest) => {
    const { data } = await api.post<BizOne<BizExpense>>('/biz/expenses', payload, bizOpts(apiKey));
    return data.data;
  },
  updateExpense: async (apiKey: string, id: string, payload: Partial<CreateBizExpenseRequest>) => {
    const { data } = await api.put<BizOne<BizExpense>>(`/biz/expenses/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteExpense: async (apiKey: string, id: string) => {
    await api.delete(`/biz/expenses/${id}`, bizOpts(apiKey));
  },
  convertExpensesToInvoice: async (apiKey: string, payload: { user_id: string; expense_ids: string[] }) => {
    const { data } = await api.post<BizOne<BizInvoice>>('/biz/expenses/convert', payload, bizOpts(apiKey));
    return data.data;
  },
  getExpenseSummary: async (apiKey: string, params?: { from?: string; to?: string }) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/expenses/summary', { ...bizOpts(apiKey), params });
    return data.data;
  },

  // ── Customer tools ──
  getUserStatement: async (apiKey: string, userId: string) => {
    const { data } = await api.get<BizOne<BizStatement>>(`/biz/users/${userId}/statement`, bizOpts(apiKey));
    return data.data;
  },
  adjustUserBalance: async (apiKey: string, userId: string, payload: { amount_paisa: number; reason?: string }) => {
    const { data } = await api.post<BizOne<{ balance_paisa: number }>>(
      `/biz/users/${userId}/balance`,
      payload,
      bizOpts(apiKey),
    );
    return data.data;
  },
  createPortalLink: async (apiKey: string, userId: string) => {
    const { data } = await api.post<BizOne<BizPortalLink>>(`/biz/users/${userId}/portal-link`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Analytics ──
  analyticsRevenue: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/revenue', bizOpts(apiKey));
    return data.data;
  },
  analyticsSubscriptions: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/subscriptions', bizOpts(apiKey));
    return data.data;
  },
  analyticsInvoices: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/invoices', bizOpts(apiKey));
    return data.data;
  },
  analyticsAging: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/aging', bizOpts(apiKey));
    return data.data;
  },
  analyticsCustomers: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>[]>>('/biz/analytics/customers', bizOpts(apiKey));
    return data.data;
  },
  analyticsDunning: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/dunning', bizOpts(apiKey));
    return data.data;
  },
  analyticsRevenueRecognition: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/revenue-recognition', bizOpts(apiKey));
    return data.data;
  },
  analyticsTaxSummary: async (apiKey: string) => {
    const { data } = await api.get<BizOne<Record<string, unknown>>>('/biz/analytics/tax-summary', bizOpts(apiKey));
    return data.data;
  },

  // ── Connectors ──
  listConnectors: async (apiKey: string) => {
    const { data } = await api.get<BizOne<BizConnector[]>>('/biz/connectors', bizOpts(apiKey));
    return data.data;
  },
  createConnector: async (apiKey: string, payload: CreateBizConnectorRequest) => {
    const { data } = await api.post<BizOne<BizConnector>>('/biz/connectors', payload, bizOpts(apiKey));
    return data.data;
  },
  updateConnector: async (apiKey: string, id: string, payload: UpdateBizConnectorRequest) => {
    const { data } = await api.put<BizOne<BizConnector>>(`/biz/connectors/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteConnector: async (apiKey: string, id: string) => {
    await api.delete(`/biz/connectors/${id}`, bizOpts(apiKey));
  },
};

export default api;
