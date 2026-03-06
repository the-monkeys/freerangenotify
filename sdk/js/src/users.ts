import { HttpClient } from './client';
import type {
  CreateUserParams,
  UpdateUserParams,
  User,
  UserListResponse,
  BulkCreateUsersResult,
  AddDeviceParams,
  Device,
  Preferences,
} from './types';

export class UsersClient {
  constructor(private readonly http: HttpClient) {}

  async create(params: CreateUserParams): Promise<User> {
    return this.http.request<User>('POST', '/users/', params);
  }

  async bulkCreate(users: CreateUserParams[]): Promise<BulkCreateUsersResult> {
    return this.http.request<BulkCreateUsersResult>('POST', '/users/bulk', { users });
  }

  async get(userId: string): Promise<User> {
    return this.http.request<User>('GET', `/users/${userId}`);
  }

  async update(userId: string, params: UpdateUserParams): Promise<User> {
    return this.http.request<User>('PUT', `/users/${userId}`, params);
  }

  async delete(userId: string): Promise<void> {
    await this.http.request('DELETE', `/users/${userId}`);
  }

  async list(page?: number, pageSize?: number): Promise<UserListResponse> {
    const query: Record<string, string | undefined> = {};
    if (page) query.page = String(page);
    if (pageSize) query.page_size = String(pageSize);
    return this.http.request<UserListResponse>('GET', '/users/', undefined, query);
  }

  // ── Devices ──

  async addDevice(userId: string, params: AddDeviceParams): Promise<Device> {
    return this.http.request<Device>('POST', `/users/${userId}/devices`, params);
  }

  async getDevices(userId: string): Promise<Device[]> {
    const result = await this.http.request<{ devices: Device[] }>(
      'GET',
      `/users/${userId}/devices`,
    );
    return result.devices;
  }

  async removeDevice(userId: string, deviceId: string): Promise<void> {
    await this.http.request('DELETE', `/users/${userId}/devices/${deviceId}`);
  }

  // ── Preferences ──

  async getPreferences(userId: string): Promise<Preferences> {
    const result = await this.http.request<{ preferences: Preferences }>(
      'GET',
      `/users/${userId}/preferences`,
    );
    return result.preferences;
  }

  async updatePreferences(
    userId: string,
    prefs: Partial<Preferences>,
  ): Promise<Preferences> {
    const result = await this.http.request<{ preferences: Preferences }>(
      'PUT',
      `/users/${userId}/preferences`,
      prefs,
    );
    return result.preferences;
  }

  // ── Subscriber Hash ──

  async getSubscriberHash(userId: string): Promise<string> {
    const result = await this.http.request<{ subscriber_hash: string }>(
      'GET',
      `/users/${userId}/subscriber-hash`,
    );
    return result.subscriber_hash;
  }
}
