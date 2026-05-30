import { HttpClient } from './client';
import { FreeRangeNotifyError } from './errors';

/**
 * File metadata as returned by the `/v1/files` endpoints.
 *
 * `expires_at` is omitted for pinned (never-expire) files. SHA256 lets you
 * verify integrity end-to-end if you persist file IDs in your own DB.
 */
export interface FileObject {
    file_id: string;
    name: string;
    size: number;
    mime_type: string;
    sha256: string;
    expires_at?: string;
    created_at: string;
}

export interface FileListResponse {
    files: FileObject[];
    total: number;
}

export interface SignedURL {
    url: string;
    expires_at: string;
}

/**
 * Body of `files.upload()`. `data` is required; `name` and `mime_type` default
 * to `"file"` and `"application/octet-stream"` respectively when omitted.
 *
 * In Node 18+ and modern browsers `data` accepts a `Blob`, `File`,
 * `ArrayBuffer`, `Uint8Array`, or `string`.
 */
export interface UploadFileParams {
    data: Blob | ArrayBuffer | Uint8Array | string;
    name?: string;
    mime_type?: string;
}

export interface ListFilesOptions {
    limit?: number;
    offset?: number;
}

/**
 * Client for the `/v1/files` API.
 *
 * Use `upload()` for any file larger than ~10 MB or that you reuse across
 * many notifications (e.g. invoices). The returned `file_id` can then be
 * referenced from a notification's attachments:
 *
 * ```ts
 * const obj = await client.files.upload({
 *   data: invoicePdfBlob,
 *   name: 'invoice-2026-05.pdf',
 *   mime_type: 'application/pdf',
 * });
 *
 * await client.notifications.send({
 *   user_id: userId,
 *   channel: 'email',
 *   template: 'invoice_email',
 *   content: {
 *     title: 'Your May invoice',
 *     body:  'Your invoice is attached.',
 *     attachments: [{ type: 'file', file_id: obj.file_id, name: obj.name }],
 *   },
 * });
 * ```
 */
export class FilesClient {
    constructor(private readonly http: HttpClient, private readonly apiKey: string, private readonly baseURL: string) { }

    async upload(params: UploadFileParams): Promise<FileObject> {
        if (params.data === undefined || params.data === null) {
            throw new Error('files.upload: data is required');
        }
        const name = (params.name ?? '').trim() || 'file';
        const mimeType = (params.mime_type ?? '').trim() || 'application/octet-stream';

        // Coerce arbitrary inputs into a Blob with the desired MIME type so the
        // resulting multipart part carries an accurate Content-Type. We build
        // FormData rather than hand-crafting the body so platform-specific
        // boundary handling is delegated to fetch.
        const blob = toBlob(params.data, mimeType);
        const form = new FormData();
        form.append('file', blob, name);

        const res = await fetch(this.baseURL + '/files', {
            method: 'POST',
            headers: { Authorization: `Bearer ${this.apiKey}` },
            body: form,
        });
        if (!res.ok) {
            const text = await res.text().catch(() => '');
            throw new FreeRangeNotifyError(res.status, text);
        }
        return res.json() as Promise<FileObject>;
    }

    async get(fileId: string): Promise<FileObject> {
        return this.http.request<FileObject>('GET', `/files/${encodeURIComponent(fileId)}`);
    }

    async list(opts: ListFilesOptions = {}): Promise<FileListResponse> {
        return this.http.request<FileListResponse>('GET', '/files', undefined, {
            limit: opts.limit !== undefined ? String(opts.limit) : undefined,
            offset: opts.offset !== undefined ? String(opts.offset) : undefined,
        });
    }

    async delete(fileId: string): Promise<void> {
        await this.http.request<void>('DELETE', `/files/${encodeURIComponent(fileId)}`);
    }

    /**
     * Streams the file bytes back. The returned Response lets you consume the
     * body via `.blob()`, `.arrayBuffer()`, `.body` (stream), or `.text()`.
     */
    async content(fileId: string): Promise<Response> {
        const res = await fetch(this.baseURL + `/files/${encodeURIComponent(fileId)}/content`, {
            method: 'GET',
            headers: { Authorization: `Bearer ${this.apiKey}` },
        });
        if (!res.ok) {
            const text = await res.text().catch(() => '');
            throw new FreeRangeNotifyError(res.status, text);
        }
        return res;
    }

    /**
     * Mints a short-lived signed URL suitable for handing to an end-user or
     * third party. TTL is configured server-side (default 15 minutes).
     */
    async downloadURL(fileId: string): Promise<SignedURL> {
        return this.http.request<SignedURL>('GET', `/files/${encodeURIComponent(fileId)}/download-url`);
    }
}

function toBlob(
    data: Blob | ArrayBuffer | Uint8Array | string,
    mimeType: string,
): Blob {
    if (typeof Blob !== 'undefined' && data instanceof Blob) {
        // Preserve caller-supplied type when present; otherwise tag ours.
        return data.type ? data : new Blob([data], { type: mimeType });
    }
    if (data instanceof ArrayBuffer) {
        return new Blob([data], { type: mimeType });
    }
    if (ArrayBuffer.isView(data)) {
        // Copy to a fresh ArrayBuffer view to satisfy the BlobPart type
        // (which excludes SharedArrayBuffer-backed views).
        const view = data as ArrayBufferView;
        return new Blob([new Uint8Array(view.buffer as ArrayBuffer, view.byteOffset, view.byteLength)], { type: mimeType });
    }
    return new Blob([data as string], { type: mimeType });
}
