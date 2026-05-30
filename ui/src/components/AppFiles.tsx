import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { Copy, Download, Loader2, RefreshCw, Trash2, Upload } from 'lucide-react';
import { filesAPI } from '../services/api';
import type { FileObject } from '../types';
import { extractErrorMessage } from '../lib/utils';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Badge } from './ui/badge';
import { Pagination } from './Pagination';
import ConfirmDeleteDialog from './ConfirmDeleteDialog';

interface AppFilesProps {
    apiKey: string;
}

const PAGE_SIZE = 20;

function formatBytes(n: number): string {
    if (n <= 0) return '0 B';
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(2)} MB`;
    return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

function formatDate(iso?: string): string {
    if (!iso) return '\u2014';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString();
}

const AppFiles: React.FC<AppFilesProps> = ({ apiKey }) => {
    const [files, setFiles] = useState<FileObject[]>([]);
    const [total, setTotal] = useState(0);
    const [loading, setLoading] = useState(false);
    const [page, setPage] = useState(1);
    const [uploading, setUploading] = useState(false);
    const [uploadProgress, setUploadProgress] = useState(0);
    const [search, setSearch] = useState('');
    const [pendingDelete, setPendingDelete] = useState<FileObject | null>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const res = await filesAPI.list(apiKey, {
                limit: PAGE_SIZE,
                offset: (page - 1) * PAGE_SIZE,
            });
            setFiles(res.files ?? []);
            setTotal(res.total ?? 0);
        } catch (e) {
            toast.error('Failed to load files: ' + extractErrorMessage(e));
        } finally {
            setLoading(false);
        }
    }, [apiKey, page]);

    useEffect(() => {
        void load();
    }, [load]);

    const filtered = useMemo(() => {
        const q = search.trim().toLowerCase();
        if (!q) return files;
        return files.filter(
            (f) =>
                f.name.toLowerCase().includes(q) ||
                f.file_id.toLowerCase().includes(q) ||
                f.mime_type.toLowerCase().includes(q),
        );
    }, [files, search]);

    const handleUpload = async (file: File) => {
        setUploading(true);
        setUploadProgress(0);
        try {
            const obj = await filesAPI.upload(apiKey, file, (p) => setUploadProgress(p));
            toast.success(`Uploaded ${obj.name}`);
            // jump to page 1 and refresh
            if (page !== 1) {
                setPage(1);
            } else {
                await load();
            }
        } catch (e) {
            toast.error('Upload failed: ' + extractErrorMessage(e));
        } finally {
            setUploading(false);
            setUploadProgress(0);
        }
    };

    const handleCopyId = async (id: string) => {
        try {
            await navigator.clipboard.writeText(id);
            toast.success('File ID copied');
        } catch {
            toast.error('Clipboard unavailable');
        }
    };

    const handleDownload = async (f: FileObject) => {
        try {
            const signed = await filesAPI.signedDownloadUrl(apiKey, f.file_id);
            // Open in a new tab so the browser can stream the bytes directly.
            window.open(signed.url, '_blank', 'noopener,noreferrer');
        } catch (e) {
            toast.error('Could not mint download URL: ' + extractErrorMessage(e));
        }
    };

    const handleConfirmDelete = async () => {
        if (!pendingDelete) return;
        try {
            await filesAPI.delete(apiKey, pendingDelete.file_id);
            toast.success(`Deleted ${pendingDelete.name}`);
            setPendingDelete(null);
            await load();
        } catch (e) {
            toast.error('Delete failed: ' + extractErrorMessage(e));
        }
    };

    return (
        <Card>
            <CardHeader className="flex flex-row items-start justify-between gap-4">
                <div>
                    <CardTitle>Files</CardTitle>
                    <p className="text-sm text-muted-foreground mt-1">
                        Managed files. Reference any row's <code className="font-mono">file_id</code> from a notification's
                        attachments, or share via a short-lived signed download URL.
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    <Button type="button" size="sm" variant="outline" onClick={() => void load()} disabled={loading}>
                        <RefreshCw className={`h-3.5 w-3.5 mr-1 ${loading ? 'animate-spin' : ''}`} />
                        Refresh
                    </Button>
                    <input
                        ref={fileInputRef}
                        type="file"
                        className="hidden"
                        onChange={(e) => {
                            const f = e.target.files?.[0];
                            if (f) void handleUpload(f);
                            e.target.value = '';
                        }}
                    />
                    <Button
                        type="button"
                        size="sm"
                        onClick={() => fileInputRef.current?.click()}
                        disabled={uploading}
                    >
                        {uploading ? (
                            <><Loader2 className="h-3.5 w-3.5 mr-1 animate-spin" />Uploading {(uploadProgress * 100).toFixed(0)}%</>
                        ) : (
                            <><Upload className="h-3.5 w-3.5 mr-1" />Upload file</>
                        )}
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                <div className="mb-3">
                    <Input
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="Filter by name, ID, or MIME type"
                        className="max-w-sm h-9 text-sm"
                    />
                </div>

                <div className="rounded-md border overflow-hidden">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>File ID</TableHead>
                                <TableHead>Size</TableHead>
                                <TableHead>Type</TableHead>
                                <TableHead>Uploaded</TableHead>
                                <TableHead>Expires</TableHead>
                                <TableHead className="text-right">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {loading && filtered.length === 0 && (
                                <TableRow>
                                    <TableCell colSpan={7} className="text-center text-sm text-muted-foreground py-8">
                                        Loading\u2026
                                    </TableCell>
                                </TableRow>
                            )}
                            {!loading && filtered.length === 0 && (
                                <TableRow>
                                    <TableCell colSpan={7} className="text-center text-sm text-muted-foreground py-8">
                                        No files uploaded yet. Click <strong>Upload file</strong> to add one.
                                    </TableCell>
                                </TableRow>
                            )}
                            {filtered.map((f) => (
                                <TableRow key={f.file_id}>
                                    <TableCell className="font-medium truncate max-w-[220px]" title={f.name}>{f.name}</TableCell>
                                    <TableCell>
                                        <code className="font-mono text-[11px] text-muted-foreground">{f.file_id}</code>
                                    </TableCell>
                                    <TableCell>{formatBytes(f.size)}</TableCell>
                                    <TableCell>
                                        <Badge variant="outline" className="text-[10px] font-mono">{f.mime_type}</Badge>
                                    </TableCell>
                                    <TableCell className="text-xs text-muted-foreground">{formatDate(f.created_at)}</TableCell>
                                    <TableCell className="text-xs text-muted-foreground">
                                        {f.expires_at ? formatDate(f.expires_at) : <Badge variant="secondary" className="text-[10px]">Pinned</Badge>}
                                    </TableCell>
                                    <TableCell className="text-right">
                                        <div className="inline-flex gap-1">
                                            <Button
                                                type="button"
                                                size="sm"
                                                variant="ghost"
                                                title="Copy file ID"
                                                onClick={() => void handleCopyId(f.file_id)}
                                            >
                                                <Copy className="h-3.5 w-3.5" />
                                            </Button>
                                            <Button
                                                type="button"
                                                size="sm"
                                                variant="ghost"
                                                title="Download"
                                                onClick={() => void handleDownload(f)}
                                            >
                                                <Download className="h-3.5 w-3.5" />
                                            </Button>
                                            <Button
                                                type="button"
                                                size="sm"
                                                variant="ghost"
                                                title="Delete"
                                                onClick={() => setPendingDelete(f)}
                                            >
                                                <Trash2 className="h-3.5 w-3.5 text-red-500" />
                                            </Button>
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </div>

                {total > PAGE_SIZE && (
                    <div className="mt-4 flex justify-end">
                        <Pagination
                            currentPage={page}
                            totalItems={total}
                            pageSize={PAGE_SIZE}
                            onPageChange={setPage}
                        />
                    </div>
                )}
            </CardContent>

            <ConfirmDeleteDialog
                open={!!pendingDelete}
                onOpenChange={(open) => { if (!open) setPendingDelete(null); }}
                title="Delete file?"
                description={
                    pendingDelete
                        ? `Permanently delete "${pendingDelete.name}" (${pendingDelete.file_id})? Notifications that reference this file_id will fail to deliver.`
                        : ''
                }
                onConfirm={() => void handleConfirmDelete()}
            />
        </Card>
    );
};

export default AppFiles;
