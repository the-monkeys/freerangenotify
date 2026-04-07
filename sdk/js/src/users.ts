import { HttpClient } from './client';
import type {
  CreateUserParams,
  UpdateUserParams,
  User,
  UserListResponse,
  BulkCreateUsersResult,
  BulkCreateUsersParams,
  AddDeviceParams,
  Device,
  Preferences,
} from './types';

export class UsersClient {
  constructor(private readonly http: HttpClient) { }

  async create(params: CreateUserParams): Promise<User> {
    return this.http.request<User>('POST', '/users/', params);
  }

  /** Register multiple users. Use skip_existing or upsert to handle duplicates. */
  async bulkCreate(params: BulkCreateUsersParams): Promise<BulkCreateUsersResult> {
    return this.http.request<BulkCreateUsersResult>('POST', '/users/bulk', params);
  }

  /** Retrieve a user by internal UUID or external_id. */
  async get(identifier: string): Promise<User> {
    return this.http.request<User>('GET', `/users/${identifier}`);
  }

  /** Retrieve a user by their external_id. */
  async getByExternalId(externalId: string): Promise<User> {
    return this.http.request<User>('GET', `/users/by-external-id/${externalId}`);
  }

  /** Update a user. Accepts internal UUID or external_id. */
  async update(identifier: string, params: UpdateUserParams): Promise<User> {
    return this.http.request<User>('PUT', `/users/${identifier}`, params);
  }

  /** Delete a user. Accepts internal UUID or external_id. */
  async delete(identifier: string): Promise<void> {
    await this.http.request('DELETE', `/users/${identifier}`);
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
