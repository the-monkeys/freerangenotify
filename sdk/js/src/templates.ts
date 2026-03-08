import { HttpClient } from './client';
import type {
    CreateTemplateParams,
    UpdateTemplateParams,
    CreateVersionParams,
    CloneTemplateParams,
    Template,
    TemplateListResponse,
    ListTemplatesOptions,
    TemplateDiff,
    ControlsResponse,
    ControlValues,
} from './types';

export class TemplatesClient {
    constructor(private readonly http: HttpClient) { }

    async create(params: CreateTemplateParams): Promise<Template> {
        return this.http.request<Template>('POST', '/templates/', params);
    }

    async get(templateId: string): Promise<Template> {
        return this.http.request<Template>('GET', `/templates/${templateId}`);
    }

    async update(templateId: string, params: UpdateTemplateParams): Promise<Template> {
        return this.http.request<Template>('PUT', `/templates/${templateId}`, params);
    }

    async delete(templateId: string): Promise<void> {
        await this.http.request('DELETE', `/templates/${templateId}`);
    }

    async list(opts?: ListTemplatesOptions): Promise<TemplateListResponse> {
        const query: Record<string, string | undefined> = {};
        if (opts?.appId) query.app_id = opts.appId;
        if (opts?.channel) query.channel = opts.channel;
        if (opts?.name) query.name = opts.name;
        if (opts?.status) query.status = opts.status;
        if (opts?.locale) query.locale = opts.locale;
        if (opts?.limit) query.limit = String(opts.limit);
        if (opts?.offset) query.offset = String(opts.offset);

        return this.http.request<TemplateListResponse>('GET', '/templates/', undefined, query);
    }

    // ── Library ──

    async getLibrary(category?: string): Promise<Template[]> {
        const query: Record<string, string | undefined> = {};
        if (category) query.category = category;

        const result = await this.http.request<{ templates: Template[] }>(
            'GET',
            '/templates/library',
            undefined,
            query,
        );
        return result.templates;
    }

    async cloneFromLibrary(name: string, appId: string): Promise<Template> {
        return this.http.request<Template>('POST', `/templates/library/${name}/clone`, {
            app_id: appId,
        } as CloneTemplateParams);
    }

    // ── Versioning ──

    async getVersions(appId: string, name: string): Promise<Template[]> {
        const result = await this.http.request<{ versions: Template[] }>(
            'GET',
            `/templates/${appId}/${name}/versions`,
        );
        return result.versions;
    }

    async createVersion(
        appId: string,
        name: string,
        params: CreateVersionParams,
    ): Promise<Template> {
        return this.http.request<Template>(
            'POST',
            `/templates/${appId}/${name}/versions`,
            params,
        );
    }

    // ── Rollback ──

    async rollback(
        templateId: string,
        version: number,
        updatedBy: string,
    ): Promise<Template> {
        return this.http.request<Template>('POST', `/templates/${templateId}/rollback`, {
            version,
            updated_by: updatedBy,
        });
    }

    // ── Diff ──

    async diff(
        templateId: string,
        fromVersion: number,
        toVersion: number,
    ): Promise<TemplateDiff> {
        return this.http.request<TemplateDiff>(
            'GET',
            `/templates/${templateId}/diff`,
            undefined,
            { from: String(fromVersion), to: String(toVersion) },
        );
    }

    // ── Render ──

    async render(
        templateId: string,
        data: Record<string, unknown>,
    ): Promise<string> {
        const result = await this.http.request<{ rendered_body: string }>(
            'POST',
            `/templates/${templateId}/render`,
            { data },
        );
        return result.rendered_body;
    }

    // ── Send Test ──

    async sendTest(
        templateId: string,
        toEmail: string,
        sampleData?: Record<string, unknown>,
    ): Promise<void> {
        await this.http.request('POST', `/templates/${templateId}/test`, {
            to_email: toEmail,
            sample_data: sampleData,
        });
    }

    // ── Content Controls ──

    /** Get the template's control definitions and current values. */
    async getControls(templateId: string): Promise<ControlsResponse> {
        return this.http.request<ControlsResponse>('GET', `/templates/${templateId}/controls`);
    }

    /** Save validated control values for a template. */
    async updateControls(templateId: string, values: ControlValues): Promise<void> {
        await this.http.request('PUT', `/templates/${templateId}/controls`, values);
    }
}
