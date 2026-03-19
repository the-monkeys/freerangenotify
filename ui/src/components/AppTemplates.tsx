import React, { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useApiQuery } from '../hooks/use-api-query';
import { templatesAPI } from '../services/api';
import type { Template, CreateTemplateRequest, TemplateVersion } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from './ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Pagination } from './Pagination';
import TemplateEditor from './TemplateEditor';
import TemplateDiffViewer from './templates/TemplateDiffViewer';
import TemplateTestPanel from './templates/TemplateTestPanel';
import TemplateControlsPanel from './templates/TemplateControlsPanel';
import EditablePreviewPanel from './templates/EditablePreviewPanel';
import SkeletonTable from './SkeletonTable';
import ConfirmDeleteDialog from './ConfirmDeleteDialog';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';
import { ChevronsUpDown, Copy, Eye, FlaskConical, GitCompareArrows, Pencil, Plus, Save, Settings2, Trash2 } from 'lucide-react';

interface AppTemplatesProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
}

const AppTemplates: React.FC<AppTemplatesProps> = ({ appId, apiKey, webhooks }) => {
    const navigate = useNavigate();
    const [showAddForm, setShowAddForm] = useState(false);
    const [editingTemplate, setEditingTemplate] = useState<Template | null>(null);
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);

    const offset = (page - 1) * pageSize;
    const {
        data: templatesData,
        loading,
        refetch: fetchTemplates
    } = useApiQuery(
        () => templatesAPI.list(apiKey, pageSize, offset),
        [apiKey, pageSize, offset],
        {
            cacheKey: `templates-${appId}-${offset}`,
            staleTime: 60000 // 1 minute
        }
    );

    const templates = useMemo(() => templatesData?.templates || [], [templatesData]);
    const totalCount = useMemo(() => templatesData?.total || 0, [templatesData]);
    const [formData, setFormData] = useState<CreateTemplateRequest>({
        app_id: appId,
        name: '',
        channel: 'email',
        webhook_target: '',
        subject: '',
        body: '',
        description: '',
        variables: []
    });

    const [varInput, setVarInput] = useState('');

    // Preview state
    const [activePreviews, setActivePreviews] = useState<Record<string, { data: string, rendered: string, loading: boolean }>>({});
    // Collapsed body state
    const [expandedBodies, setExpandedBodies] = useState<Record<string, boolean>>({});

    // Preview slide panel state
    const [slidePreview, setSlidePreview] = useState<{ templateId: string; templateName: string; channel: string } | null>(null);

    // Version history state
    const [versionHistoryTemplate, setVersionHistoryTemplate] = useState<Template | null>(null);
    const [versions, setVersions] = useState<TemplateVersion[]>([]);
    const [versionsLoading, setVersionsLoading] = useState(false);
    const [savingVersion, setSavingVersion] = useState(false);

    // Phase 3: Template advanced feature state
    const [diffTemplate, setDiffTemplate] = useState<Template | null>(null);
    const [testTemplate, setTestTemplate] = useState<Template | null>(null);
    const [controlsTemplate, setControlsTemplate] = useState<Template | null>(null);
    const [rollbackTarget, setRollbackTarget] = useState<{ template: Template; version: TemplateVersion } | null>(null);
    const [rollbackLoading, setRollbackLoading] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState<Template | null>(null);
    const [deleteLoading, setDeleteLoading] = useState(false);
    const [savingDefaults, setSavingDefaults] = useState<Record<string, boolean>>({});
    const [expandedTemplateVariables, setExpandedTemplateVariables] = useState<Record<string, boolean>>({});
    const [showAllFormVariables, setShowAllFormVariables] = useState(false);

    const MAX_INLINE_TEMPLATE_VARIABLES = 12;
    const MAX_INLINE_FORM_VARIABLES = 20;

    const getPreviewStorageKey = (templateId: string) => `frn:template-preview-data:${appId}:${templateId}`;

    const updatePreviewData = (templateId: string, data: string) => {
        setActivePreviews((prev) => ({
            ...prev,
            [templateId]: { ...(prev[templateId] || { rendered: '', loading: false }), data }
        }));
        try {
            localStorage.setItem(getPreviewStorageKey(templateId), data);
        } catch {
            // Ignore storage failures (private mode/quota).
        }
    };

    const updatePreviewVariable = (templateId: string, variable: string, value: string) => {
        const current = activePreviews[templateId]?.data || '{}';
        try {
            const parsed = JSON.parse(current) as Record<string, any>;
            const next = { ...parsed, [variable]: value };
            updatePreviewData(templateId, JSON.stringify(next, null, 2));
        } catch {
            // Keep current data untouched when JSON is invalid.
        }
    };

    const getVisibleTemplateVariables = (tmpl: Template): string[] => {
        const vars = tmpl.variables || [];
        if (expandedTemplateVariables[tmpl.id]) return vars;
        return vars.slice(0, MAX_INLINE_TEMPLATE_VARIABLES);
    };

    const getVisibleFormVariables = (): string[] => {
        const vars = formData.variables || [];
        if (showAllFormVariables) return vars;
        return vars.slice(0, MAX_INLINE_FORM_VARIABLES);
    };

    const formatChannelLabel = (channel: string): string => {
        if (channel === 'in_app') return 'In-App';
        if (channel === 'sse') return 'SSE';
        return channel.charAt(0).toUpperCase() + channel.slice(1);
    };


    const resetForm = () => {
        setFormData({
            app_id: appId,
            name: '',
            channel: 'email',
            webhook_target: '',
            subject: '',
            body: '',
            description: '',
            variables: []
        });
        setEditingTemplate(null);
        setVarInput('');
    };

    const handleCreateTemplate = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            if (editingTemplate) {
                await templatesAPI.update(apiKey, editingTemplate.id, {
                    description: formData.description,
                    webhook_target: formData.webhook_target,
                    subject: formData.subject,
                    body: formData.body,
                    variables: formData.variables,
                });
                toast.success('Template updated successfully!');
            } else {
                await templatesAPI.create(apiKey, { ...formData, app_id: appId });
                toast.success('Template created successfully!');
            }
            setShowAddForm(false);
            resetForm();
            fetchTemplates();
        } catch (error) {
            console.error('Failed to save template:', error);
            toast.error(extractErrorMessage(error, editingTemplate ? 'Failed to update template' : 'Failed to create template'));
        }
    };

    const handleEditTemplate = (tmpl: Template) => {
        setEditingTemplate(tmpl);
        setFormData({
            app_id: appId,
            name: tmpl.name,
            channel: tmpl.channel as any,
            webhook_target: tmpl.webhook_target || '',
            subject: tmpl.subject || '',
            body: tmpl.body,
            description: tmpl.description || '',
            variables: tmpl.variables || [],
        });
        setShowAddForm(true);
    };

    const handleAddVariable = () => {
        if (varInput && formData.variables && !formData.variables.includes(varInput)) {
            setFormData({ ...formData, variables: [...formData.variables, varInput] });
            setVarInput('');
        }
    };

    const handleDeleteTemplate = async (id: string) => {
        const target = templates.find((tmpl) => tmpl.id === id);
        if (!target) return;
        setDeleteTarget(target);
    };

    const handleConfirmDeleteTemplate = async () => {
        if (!deleteTarget) return;
        setDeleteLoading(true);
        try {
            await templatesAPI.delete(apiKey, deleteTarget.id);
            fetchTemplates();
            setDeleteTarget(null);
        } catch (error) {
            console.error('Failed to delete template:', error);
        } finally {
            setDeleteLoading(false);
        }
    };

    const getDefaultPreviewData = (tmpl: Template): string => {
        let defaultData = '{}';
        let hasPersistedData = false;

        try {
            const persisted = localStorage.getItem(getPreviewStorageKey(tmpl.id));
            if (persisted) {
                defaultData = persisted;
                hasPersistedData = true;
            }
        } catch {
            // Ignore storage failures and continue with generated defaults.
        }

        if (!hasPersistedData && tmpl.metadata?.sample_data) {
            return JSON.stringify(tmpl.metadata.sample_data, null, 2);
        }

        if (!hasPersistedData && tmpl.variables?.length) {
            const generated: Record<string, string> = {};
            for (const v of tmpl.variables) {
                generated[v] = v;
            }
            return JSON.stringify(generated, null, 2);
        }

        return defaultData;
    };

    const openPreviewDialog = async (tmpl: Template) => {
        const tmplId = tmpl.id;
        const previewData = activePreviews[tmplId]?.data || getDefaultPreviewData(tmpl);

        setActivePreviews((prev) => ({
            ...prev,
            [tmplId]: {
                ...(prev[tmplId] || { rendered: '', loading: false }),
                data: previewData,
            },
        }));

        setSlidePreview({
            templateId: tmpl.id,
            templateName: tmpl.name,
            channel: tmpl.channel,
        });

        let parsedData = {};
        try {
            parsedData = JSON.parse(previewData);
        } catch {
            toast.error('Invalid JSON data for preview');
            return;
        }

        setActivePreviews((prev) => ({
            ...prev,
            [tmplId]: {
                ...(prev[tmplId] || { data: previewData, rendered: '' }),
                data: prev[tmplId]?.data ?? previewData,
                loading: true,
            },
        }));

        try {
            const resp = await templatesAPI.render(apiKey, tmplId, { data: parsedData, editable: true });
            setActivePreviews((prev) => ({
                ...prev,
                [tmplId]: {
                    ...(prev[tmplId] || { data: previewData, rendered: '' }),
                    data: prev[tmplId]?.data ?? previewData,
                    rendered: resp.rendered_body,
                    loading: false,
                },
            }));
        } catch (error) {
            console.error('Failed to render preview:', error);
            toast.error('Failed to render preview');
            setActivePreviews((prev) => ({
                ...prev,
                [tmplId]: {
                    ...(prev[tmplId] || { data: previewData, rendered: '' }),
                    data: prev[tmplId]?.data ?? previewData,
                    loading: false,
                },
            }));
        }
    };

    const handleRenderPreview = async (tmplId: string) => {
        const preview = activePreviews[tmplId];
        if (!preview) return;

        let parsedData = {};
        try {
            parsedData = JSON.parse(preview.data);
        } catch (e) {
            toast.error('Invalid JSON data');
            return;
        }

        setActivePreviews((prev) => ({
            ...prev,
            [tmplId]: { ...preview, loading: true }
        }));

        try {
            const resp = await templatesAPI.render(apiKey, tmplId, { data: parsedData, editable: true });
            setActivePreviews((prev) => ({
                ...prev,
                [tmplId]: { ...preview, rendered: resp.rendered_body, loading: false }
            }));
        } catch (error) {
            console.error('Failed to render preview:', error);
            toast.error('Failed to render preview');
            setActivePreviews((prev) => ({
                ...prev,
                [tmplId]: { ...preview, loading: false }
            }));
        }
    };

    const handleSavePreviewDefaults = async (tmpl: Template) => {
        const preview = activePreviews[tmpl.id];
        if (!preview) return;

        let parsedData: Record<string, any> = {};
        try {
            parsedData = JSON.parse(preview.data || '{}');
        } catch {
            toast.error('Preview JSON must be valid before saving defaults');
            return;
        }

        setSavingDefaults((prev) => ({ ...prev, [tmpl.id]: true }));
        try {
            await templatesAPI.update(apiKey, tmpl.id, {
                metadata: {
                    ...(tmpl.metadata || {}),
                    sample_data: parsedData,
                },
            });
            try {
                localStorage.setItem(getPreviewStorageKey(tmpl.id), JSON.stringify(parsedData, null, 2));
            } catch {
                // Ignore storage failures.
            }
            toast.success('Saved as template defaults');
            fetchTemplates();
        } catch (error) {
            toast.error(extractErrorMessage(error, 'Failed to save template defaults'));
        } finally {
            setSavingDefaults((prev) => ({ ...prev, [tmpl.id]: false }));
        }
    };

    const fetchVersionHistory = async (tmpl: Template) => {
        setVersionHistoryTemplate(tmpl);
        setVersionsLoading(true);
        try {
            const data = await templatesAPI.getVersions(apiKey, appId, tmpl.name);
            setVersions(Array.isArray(data) ? data : (data as any)?.templates || []);
        } catch (error) {
            console.error('Failed to fetch versions:', error);
            toast.error('Failed to load version history');
            setVersions([]);
        } finally {
            setVersionsLoading(false);
        }
    };

    const handleSaveVersion = async (tmpl: Template) => {
        setSavingVersion(true);
        try {
            await templatesAPI.createVersion(apiKey, appId, tmpl.name, {
                body: tmpl.body,
                subject: tmpl.subject,
                description: tmpl.description,
                variables: tmpl.variables,
            });
            toast.success(`Version saved for "${tmpl.name}"`);
            // Refresh templates to get the updated version number
            fetchTemplates();
        } catch (error: any) {
            const msg = error?.response?.data?.error || 'Failed to save version';
            toast.error(msg);
        } finally {
            setSavingVersion(false);
        }
    };

    const handleRestoreVersion = async (ver: TemplateVersion) => {
        if (!versionHistoryTemplate) return;
        // Open confirmation dialog instead of directly restoring
        setRollbackTarget({ template: versionHistoryTemplate, version: ver });
    };

    const handleConfirmRollback = async () => {
        if (!rollbackTarget) return;
        setRollbackLoading(true);
        try {
            await templatesAPI.rollback(apiKey, rollbackTarget.template.id, {
                target_version: rollbackTarget.version.version,
            });
            toast.success(`Rolled back to version ${rollbackTarget.version.version}`);
            setRollbackTarget(null);
            setVersionHistoryTemplate(null);
            fetchTemplates();
        } catch (error: any) {
            const msg = error?.response?.data?.error || 'Rollback failed';
            toast.error(msg);
        } finally {
            setRollbackLoading(false);
        }
    };

    const openDiffViewer = (tmpl: Template) => {
        // Eagerly load versions for the diff viewer
        setDiffTemplate(tmpl);
    };

    if (loading) return <SkeletonTable rows={4} columns={5} />;

    return (
        <Card className="bg-card/60 shadow-sm">
            <CardHeader className="pb-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="space-y-1">
                        <CardTitle className="text-xl">Notification Templates</CardTitle>
                        <p className="text-sm text-muted-foreground">
                            Manage templates, run previews and tests, and keep versions clean.
                        </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                        <Badge variant="outline" className="h-7 px-2 text-xs">
                            {totalCount} total
                        </Badge>
                        <Button variant="outline" size="sm" onClick={() => navigate(`/apps/${appId}/templates/library`)}>
                            Browse Library
                        </Button>
                        <Button
                            size="sm"
                            onClick={() => {
                                if (showAddForm) {
                                    setShowAddForm(false);
                                    resetForm();
                                } else {
                                    resetForm();
                                    setShowAddForm(true);
                                }
                            }}
                        >
                            {showAddForm ? 'Cancel' : 'Create Template'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent className="space-y-5">
                <div className="rounded-lg border border-border/70 bg-muted/35 p-3 text-xs text-muted-foreground">
                    <span className="font-semibold text-foreground">Template actions:</span>{' '}
                    Preview renders with sample data, Compare checks version diffs, Test sends sample delivery,
                    Controls updates safe business fields, Save Version snapshots current content,
                    Edit updates source, and Delete permanently removes a template.
                </div>

                {showAddForm && (
                    <form onSubmit={handleCreateTemplate} className="rounded-xl border border-border bg-background p-4 sm:p-5 space-y-4">
                        <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                            <h4 className="text-base font-semibold">{editingTemplate ? 'Edit Template' : 'Create Template'}</h4>
                            {editingTemplate && (
                                <Badge variant="outline" className="w-fit">Editing existing template</Badge>
                            )}
                        </div>

                        <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                            <div className="space-y-2">
                                <Label htmlFor="templateName">Template Name</Label>
                                <Input
                                    id="templateName"
                                    type="text"
                                    value={formData.name}
                                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                                    required
                                    placeholder="e.g. welcome_email"
                                    disabled={!!editingTemplate}
                                    className={editingTemplate ? 'bg-muted cursor-not-allowed' : ''}
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="channel">Channel</Label>
                                <Select
                                    value={formData.channel}
                                    onValueChange={(value) => setFormData({ ...formData, channel: value as any })}
                                    disabled={!!editingTemplate}
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="email">Email</SelectItem>
                                        {/* <SelectItem value="push">Push</SelectItem>
                                        <SelectItem value="sms">SMS</SelectItem> */}
                                        <SelectItem value="whatsapp">WhatsApp</SelectItem>
                                        <SelectItem value="webhook">Webhook</SelectItem>
                                        <SelectItem value="in_app">In-App</SelectItem>
                                        <SelectItem value="sse">SSE (Server-Sent Events)</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                        {formData.channel === 'webhook' && webhooks && Object.keys(webhooks).length > 0 && (
                            <div className="space-y-2">
                                <Label htmlFor="webhookTarget">Webhook Target</Label>
                                <Select
                                    value={formData.webhook_target || '__default__'}
                                    onValueChange={(value) => setFormData({ ...formData, webhook_target: value === '__default__' ? '' : value })}
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder="Default (Application Webhook URL)" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="__default__">Default (Application Webhook URL)</SelectItem>
                                        {Object.keys(webhooks).map(name => (
                                            <SelectItem key={name} value={name}>{name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-muted-foreground">
                                    Select a specific named webhook endpoint for this template.
                                </p>
                            </div>
                        )}
                        {formData.channel === 'email' && (
                            <div className="space-y-2">
                                <Label htmlFor="subject">Subject (for Email)</Label>
                                <Input
                                    id="subject"
                                    type="text"
                                    value={formData.subject || ''}
                                    onChange={(e) => setFormData({ ...formData, subject: e.target.value })}
                                    placeholder="Email subject"
                                />
                            </div>
                        )}
                        <div className="space-y-2">
                            <Label htmlFor="body">Body / Content</Label>
                            <TemplateEditor
                                content={formData.body}
                                variables={formData.variables || []}
                                onChange={(newBody) => {
                                    // Auto-detect variables like {{.var_name}}
                                    const regex = /{{\s*\.?(\w+)\s*}}/g;
                                    const matches = new Set<string>();
                                    let match;
                                    while ((match = regex.exec(newBody)) !== null) {
                                        if (match[1]) matches.add(match[1]);
                                    }
                                    const currentVars = new Set(formData.variables || []);
                                    for (const m of matches) currentVars.add(m);
                                    setFormData({
                                        ...formData,
                                        body: newBody,
                                        variables: Array.from(currentVars)
                                    });
                                }}
                                channel={formData.channel}
                                placeholder="Hello {{.name}}, welcome!"
                            />
                            <p className="text-xs text-muted-foreground">
                                Use <code>{'{{.variable_name}}'}</code> syntax. Detected variables will enter the list below automatically.
                            </p>
                        </div>
                        <div className="space-y-2">
                            <div className="flex items-center justify-between gap-2">
                                <Label>Variables</Label>
                                <span className="text-xs text-muted-foreground">{(formData.variables || []).length} declared</span>
                            </div>
                            <div className="flex gap-2">
                                <Input
                                    type="text"
                                    value={varInput}
                                    onChange={(e) => setVarInput(e.target.value)}
                                    placeholder="name"
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            e.preventDefault();
                                            handleAddVariable();
                                        }
                                    }}
                                />
                                <Button type="button" variant="secondary" onClick={handleAddVariable}>
                                    <Plus className="h-3.5 w-3.5" />
                                    Add
                                </Button>
                            </div>
                            <div className="rounded-lg border border-border/80 bg-muted/25 p-2.5">
                                <div className="flex max-h-36 flex-wrap gap-1.5 overflow-y-auto pr-1">
                                    {getVisibleFormVariables().map(v => (
                                        <Badge key={v} variant="outline" className="text-sm">
                                            {v}
                                            <button
                                                type="button"
                                                onClick={() => setFormData({ ...formData, variables: formData.variables?.filter(x => x !== v) || [] })}
                                                className="ml-2 text-red-600 hover:text-red-700 font-bold"
                                            >
                                                &times;
                                            </button>
                                        </Badge>
                                    ))}
                                </div>
                                {(formData.variables || []).length > MAX_INLINE_FORM_VARIABLES && (
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        className="mt-2 h-6 px-2 text-xs"
                                        onClick={() => setShowAllFormVariables((prev) => !prev)}
                                    >
                                        <ChevronsUpDown className="h-3.5 w-3.5" />
                                        {showAllFormVariables
                                            ? 'Show fewer variables'
                                            : `Show all ${(formData.variables || []).length} variables`}
                                    </Button>
                                )}
                            </div>
                            <p className="text-xs text-muted-foreground">
                                All variables used in the body should be declared for validation.
                            </p>
                        </div>
                        <div className="flex justify-end mt-6">
                            <Button type="submit" size="sm">{editingTemplate ? 'Update Template' : 'Create Template'}</Button>
                        </div>
                    </form>
                )}

                {!templates || templates.length === 0 ? (
                    <p className="text-muted-foreground text-center py-8">No templates found.</p>
                ) : (
                    <div className="space-y-4">
                        {templates.map((tmpl) => (
                            <Card key={tmpl.id} className="bg-card border-border/80 shadow-sm">
                                <CardContent className="space-y-4 pt-5">
                                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                                        <div className="min-w-0 space-y-1">
                                            <h4 className="truncate text-base font-semibold text-foreground">{tmpl.name}</h4>
                                            <p className="text-sm text-muted-foreground line-clamp-1 max-w-lg">{tmpl.description || 'No description provided.'}</p>
                                            <div className="flex flex-wrap items-center gap-2 pt-1 text-xs text-muted-foreground">
                                                <span className="font-mono">{tmpl.id}</span>
                                                <Button
                                                    type="button"
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-6 px-2"
                                                    title="Copy Template ID"
                                                    onClick={() => {
                                                        navigator.clipboard.writeText(tmpl.id);
                                                        toast.success('Template ID copied to clipboard');
                                                    }}
                                                >
                                                    <Copy className="h-3.5 w-3.5" />
                                                    Copy ID
                                                </Button>
                                            </div>
                                        </div>
                                        <div className="flex flex-wrap items-center gap-2">
                                            <Badge variant="outline" className="px-2 py-1 uppercase">
                                                {formatChannelLabel(tmpl.channel)}
                                            </Badge>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                className="h-7"
                                                onClick={() => fetchVersionHistory(tmpl)}
                                            >
                                                v{tmpl.version} History
                                            </Button>
                                        </div>
                                    </div>

                                    {tmpl.channel === 'webhook' && (
                                        <div className="rounded-md border border-border/70 bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
                                            Target: <span className="font-medium text-foreground">{tmpl.webhook_target || 'Default application webhook'}</span>
                                        </div>
                                    )}

                                    {(tmpl.variables || []).length > 0 && (
                                        <div className="rounded-md border border-border/70 bg-muted/20 p-2.5">
                                            <div className="mb-2 flex items-center justify-between gap-2">
                                                <p className="text-xs font-medium text-muted-foreground">
                                                    Variables ({(tmpl.variables || []).length})
                                                </p>
                                                {(tmpl.variables || []).length > MAX_INLINE_TEMPLATE_VARIABLES && (
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        className="h-6 px-2 text-xs"
                                                        onClick={() =>
                                                            setExpandedTemplateVariables((prev) => ({
                                                                ...prev,
                                                                [tmpl.id]: !prev[tmpl.id],
                                                            }))
                                                        }
                                                    >
                                                        <ChevronsUpDown className="h-3.5 w-3.5" />
                                                        {expandedTemplateVariables[tmpl.id]
                                                            ? 'Show fewer'
                                                            : `Show all ${(tmpl.variables || []).length}`}
                                                    </Button>
                                                )}
                                            </div>
                                            <div className="flex max-h-24 flex-wrap gap-1.5 overflow-y-auto pr-1">
                                                {getVisibleTemplateVariables(tmpl).map((v) => (
                                                    <Badge key={v} variant="outline" className="h-6 px-2 text-[11px]">
                                                        {v}
                                                    </Badge>
                                                ))}
                                            </div>
                                        </div>
                                    )}

                                    <div className="bg-muted/40 rounded-lg border border-dashed border-border/80 p-3.5 relative">
                                        <div className="mb-2 flex items-center justify-between">
                                            <div className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Template body</div>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="sm"
                                                className="h-6 px-2 text-[11px]"
                                                onClick={() => {
                                                    setExpandedBodies(prev => ({
                                                        ...prev,
                                                        [tmpl.id]: !prev[tmpl.id]
                                                    }));
                                                }}
                                            >
                                                {expandedBodies[tmpl.id] ? 'Collapse' : 'Expand'}
                                            </Button>
                                        </div>
                                        <div style={{
                                            maxHeight: expandedBodies[tmpl.id] ? 'none' : '84px',
                                            overflow: 'hidden',
                                            transition: 'max-height 0.25s ease-in-out'
                                        }}>
                                            <pre className="m-0 select-text whitespace-pre-wrap font-mono text-xs text-foreground">{tmpl.body}</pre>
                                        </div>
                                        {!expandedBodies[tmpl.id] && (
                                            <div className="pointer-events-none absolute bottom-0 left-0 right-0 h-8 rounded-b-lg bg-linear-to-t from-muted/80 to-transparent" />
                                        )}
                                    </div>

                                    <div className="flex flex-wrap gap-2">
                                        <Button
                                            onClick={() => openPreviewDialog(tmpl)}
                                            variant="secondary"
                                            size="sm"
                                            title={activePreviews[tmpl.id] ? 'Close preview panel for this template' : 'Render and preview this template with sample data'}
                                        >
                                            <Eye className="h-3.5 w-3.5" />
                                            Preview
                                        </Button>
                                        <Button
                                            onClick={() => openDiffViewer(tmpl)}
                                            variant="outline"
                                            size="sm"
                                            title="Compare two saved versions to see field-level changes"
                                        >
                                            <GitCompareArrows className="h-3.5 w-3.5" />
                                            Compare
                                        </Button>
                                        <Button
                                            onClick={() => setTestTemplate(tmpl)}
                                            variant="outline"
                                            size="sm"
                                            title="Send a test notification/email using this template"
                                        >
                                            <FlaskConical className="h-3.5 w-3.5" />
                                            Test
                                        </Button>
                                        <Button
                                            onClick={() => setControlsTemplate(tmpl)}
                                            variant="outline"
                                            size="sm"
                                            title="Edit business-safe content controls without changing raw template HTML"
                                        >
                                            <Settings2 className="h-3.5 w-3.5" />
                                            Controls
                                        </Button>
                                        <Button
                                            onClick={() => handleSaveVersion(tmpl)}
                                            variant="outline"
                                            size="sm"
                                            disabled={savingVersion}
                                            title="Create an immutable version snapshot of the current template"
                                        >
                                            <Save className="h-3.5 w-3.5" />
                                            {savingVersion ? 'Saving...' : 'Save Version'}
                                        </Button>
                                        <Button
                                            onClick={() => handleEditTemplate(tmpl)}
                                            variant="outline"
                                            size="sm"
                                            title="Edit template content, variables, and metadata"
                                        >
                                            <Pencil className="h-3.5 w-3.5" />
                                            Edit
                                        </Button>
                                        <Button
                                            onClick={() => handleDeleteTemplate(tmpl.id)}
                                            variant="destructive"
                                            size="sm"
                                            title="Permanently delete this template"
                                        >
                                            <Trash2 className="h-3.5 w-3.5" />
                                            Delete
                                        </Button>
                                    </div>
                                </CardContent>
                            </Card>
                        ))}
                    </div>
                )}

                <Pagination
                    currentPage={page}
                    totalItems={totalCount}
                    pageSize={pageSize}
                    onPageChange={setPage}
                />

                <EditablePreviewPanel
                    slidePreview={slidePreview}
                    templates={templates}
                    activePreviews={activePreviews}
                    savingDefaults={savingDefaults}
                    onClose={() => setSlidePreview(null)}
                    onRenderPreview={handleRenderPreview}
                    onSaveDefaults={handleSavePreviewDefaults}
                    onVariableEdit={updatePreviewVariable}
                />

                {/* Template Diff Viewer */}
                {diffTemplate && (
                    <TemplateDiffViewer
                        apiKey={apiKey}
                        templateId={diffTemplate.id}
                        templateName={diffTemplate.name}
                        versions={versions}
                        open={!!diffTemplate}
                        onOpenChange={(open) => { if (!open) setDiffTemplate(null); }}
                    />
                )}

                {/* Template Test Panel */}
                {testTemplate && (
                    <TemplateTestPanel
                        apiKey={apiKey}
                        template={testTemplate}
                        open={!!testTemplate}
                        onOpenChange={(open) => { if (!open) setTestTemplate(null); }}
                    />
                )}

                {/* Template Controls Panel */}
                {controlsTemplate && (
                    <TemplateControlsPanel
                        apiKey={apiKey}
                        templateId={controlsTemplate.id}
                        templateName={controlsTemplate.name}
                        open={!!controlsTemplate}
                        onOpenChange={(open) => { if (!open) setControlsTemplate(null); }}
                    />
                )}

                <ConfirmDeleteDialog
                    open={!!deleteTarget}
                    onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
                    title="Delete Template"
                    description={deleteTarget ? `Delete template \"${deleteTarget.name}\"?` : 'Delete this template?'}
                    confirmLabel="Delete"
                    confirmVariant="destructive"
                    loading={deleteLoading}
                    onConfirm={handleConfirmDeleteTemplate}
                />

                {/* Rollback Confirmation Dialog */}
                <ConfirmDeleteDialog
                    open={!!rollbackTarget}
                    onOpenChange={(open) => { if (!open) setRollbackTarget(null); }}
                    title="Confirm Rollback"
                    description={rollbackTarget ? `Roll back "${rollbackTarget.template.name}" to version ${rollbackTarget.version.version}? This will replace the current template content.` : ''}
                    confirmLabel="Rollback"
                    confirmVariant="destructive"
                    loading={rollbackLoading}
                    onConfirm={handleConfirmRollback}
                />

                {/* Version History Dialog */}
                <Dialog open={!!versionHistoryTemplate} onOpenChange={(open) => { if (!open) setVersionHistoryTemplate(null); }}>
                    <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
                        <DialogHeader>
                            <DialogTitle>Version History: {versionHistoryTemplate?.name}</DialogTitle>
                        </DialogHeader>
                        {versionsLoading ? (
                            <p className="text-muted-foreground text-center py-4">Loading versions...</p>
                        ) : versions.length === 0 ? (
                            <p className="text-muted-foreground text-center py-4">No version history found.</p>
                        ) : (
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Version</TableHead>
                                        <TableHead>Created At</TableHead>
                                        <TableHead>Subject</TableHead>
                                        <TableHead>Actions</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {versions.map(v => (
                                        <TableRow key={v.id}>
                                            <TableCell>
                                                <Badge variant="outline" className="text-xs">v{v.version}</Badge>
                                            </TableCell>
                                            <TableCell className="text-sm text-muted-foreground">
                                                {v.created_at ? new Date(v.created_at).toLocaleString() : '—'}
                                            </TableCell>
                                            <TableCell className="max-w-50 truncate text-sm text-foreground">
                                                {v.subject || '—'}
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex gap-2">
                                                    <Dialog>
                                                        <DialogTrigger asChild>
                                                            <Button variant="ghost" size="sm" className="text-xs">
                                                                <Eye className="h-3.5 w-3.5" />
                                                                Preview
                                                            </Button>
                                                        </DialogTrigger>
                                                        <DialogContent className="max-w-xl">
                                                            <DialogHeader>
                                                                <DialogTitle>Version {v.version} Preview</DialogTitle>
                                                            </DialogHeader>
                                                            <div className="bg-muted p-4 rounded border max-h-100 overflow-auto">
                                                                <pre className="whitespace-pre-wrap text-sm font-mono">{v.body}</pre>
                                                            </div>
                                                        </DialogContent>
                                                    </Dialog>
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        className="text-xs"
                                                        onClick={() => handleRestoreVersion(v)}
                                                    >
                                                        Restore
                                                    </Button>
                                                </div>
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                </TableBody>
                            </Table>
                        )}
                    </DialogContent>
                </Dialog>
            </CardContent>
        </Card>
    );
};

export default AppTemplates;
