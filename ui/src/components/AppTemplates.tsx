import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { templatesAPI } from '../services/api';
import type { Template, CreateTemplateRequest, TemplateVersion } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from './ui/dialog';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Pagination } from './Pagination';
import TemplateEditor from './TemplateEditor';
import { SlidePanel } from './ui/slide-panel';
import ConfirmDialog from './ConfirmDialog';
import TemplateDiffViewer from './templates/TemplateDiffViewer';
import TemplateTestPanel from './templates/TemplateTestPanel';
import TemplateControlsPanel from './templates/TemplateControlsPanel';
import SkeletonTable from './SkeletonTable';
import { toast } from 'sonner';

interface AppTemplatesProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
}

const AppTemplates: React.FC<AppTemplatesProps> = ({ appId, apiKey, webhooks }) => {
    const navigate = useNavigate();
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [editingTemplate, setEditingTemplate] = useState<Template | null>(null);
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);
    const [totalCount, setTotalCount] = useState(0);
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

    // Slide panel state for rendered output
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

    useEffect(() => {
        if (apiKey) {
            fetchTemplates();
        }
    }, [apiKey, page]);

    const fetchTemplates = async () => {
        setLoading(true);
        try {
            const offset = (page - 1) * pageSize;
            const result = await templatesAPI.list(apiKey, pageSize, offset);
            setTemplates(result.templates || []);
            setTotalCount(result.total || 0);
        } catch (error) {
            console.error('Failed to fetch templates:', error);
        } finally {
            setLoading(false);
        }
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
            toast.error(editingTemplate ? 'Failed to update template' : 'Failed to create template');
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
        if (!window.confirm('Delete this template?')) return;
        try {
            await templatesAPI.delete(apiKey, id);
            fetchTemplates();
        } catch (error) {
            console.error('Failed to delete template:', error);
        }
    };

    const togglePreview = (tmplId: string) => {
        if (activePreviews[tmplId]) {
            const newPreviews = { ...activePreviews };
            delete newPreviews[tmplId];
            setActivePreviews(newPreviews);
        } else {
            const tmpl = templates.find(t => t.id === tmplId);
            let defaultData = '{}';
            if (tmpl?.metadata?.sample_data) {
                defaultData = JSON.stringify(tmpl.metadata.sample_data, null, 2);
            } else if (tmpl?.name) {
                // Generate sample data from variables
                if (tmpl.variables?.length) {
                    const generated: Record<string, string> = {};
                    for (const v of tmpl.variables) {
                        generated[v] = v;
                    }
                    defaultData = JSON.stringify(generated, null, 2);
                }
            } else if (tmpl?.variables?.length) {
                const generated: Record<string, string> = {};
                for (const v of tmpl.variables) {
                    generated[v] = v;
                }
                defaultData = JSON.stringify(generated, null, 2);
            }
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { data: defaultData, rendered: '', loading: false }
            });
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

        setActivePreviews({
            ...activePreviews,
            [tmplId]: { ...preview, loading: true }
        });

        try {
            const resp = await templatesAPI.render(apiKey, tmplId, { data: parsedData });
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { ...preview, rendered: resp.rendered_body, loading: false }
            });
            // Auto-open the slide panel with the rendered output
            const tmpl = templates.find(t => t.id === tmplId);
            if (tmpl) {
                setSlidePreview({ templateId: tmplId, templateName: tmpl.name, channel: tmpl.channel });
            }
        } catch (error) {
            console.error('Failed to render preview:', error);
            toast.error('Failed to render preview');
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { ...preview, loading: false }
            });
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
        fetchVersionHistory(tmpl);
        setDiffTemplate(tmpl);
    };

    if (loading) return <SkeletonTable rows={4} columns={5} />;

    return (
        <Card>
            <CardHeader>
                <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                    <CardTitle>Notification Templates</CardTitle>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={() => navigate(`/apps/${appId}/templates/library`)}>
                            Browse Library
                        </Button>
                        <Button
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
            <CardContent>
                {showAddForm && (
                    <form onSubmit={handleCreateTemplate} className="mb-8 bg-muted p-6 rounded border border-border space-y-4">
                        <h4 className="text-lg font-semibold mb-2">{editingTemplate ? 'Edit Template' : 'Create Template'}</h4>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
                                        <SelectItem value="push">Push</SelectItem>
                                        <SelectItem value="sms">SMS</SelectItem>
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
                        <div className="space-y-2">
                            <Label htmlFor="body">Body / Content</Label>
                            <TemplateEditor
                                content={formData.body}
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
                            <Label>Variables (Must be declared to pass validation)</Label>
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
                                <Button type="button" variant="secondary" onClick={handleAddVariable}>Add</Button>
                            </div>
                            <div className="mt-2 flex gap-2 flex-wrap">
                                {(formData.variables || []).map(v => (
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
                        </div>
                        <div className="flex justify-end mt-6">
                            <Button type="submit">{editingTemplate ? 'Update Template' : 'Create Template'}</Button>
                        </div>
                    </form>
                )}

                {!templates || templates.length === 0 ? (
                    <p className="text-muted-foreground text-center py-8">No templates found.</p>
                ) : (
                    <div className="space-y-6">
                        {templates.map((tmpl) => (
                            <Card key={tmpl.id} className="bg-card border-border">
                                <CardContent className="pt-6">
                                    <div className="flex justify-between items-start mb-4">
                                        <div>
                                            <h4 className="text-lg font-semibold text-foreground mb-1">{tmpl.name}</h4>
                                            <p className="text-sm text-muted-foreground">{tmpl.description || 'No description'}</p>
                                        </div>
                                        <div className="flex items-center gap-2">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="text-xs text-muted-foreground h-6 px-2"
                                                onClick={() => fetchVersionHistory(tmpl)}
                                            >
                                                v{tmpl.version} · History
                                            </Button>
                                            <Badge variant="outline" className="text-xs uppercase bg-muted text-foreground border-foreground">
                                                {tmpl.channel}
                                            </Badge>
                                        </div>
                                    </div>

                                    {tmpl.channel === 'webhook' && (
                                        <div className="mb-2 text-sm font-semibold text-foreground">
                                            Target: <span className="text-foreground">{tmpl.webhook_target || 'Default'}</span>
                                        </div>
                                    )}

                                    <div className="mb-4 bg-muted p-4 rounded border border-dashed border-border relative">
                                        <div className="flex justify-between items-center mb-2">
                                            <div className="text-xs text-muted-foreground font-semibold">TEMPLATE BODY</div>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="sm"
                                                className="text-[11px] h-6 px-2 text-foreground font-bold"
                                                onClick={() => {
                                                    setExpandedBodies(prev => ({
                                                        ...prev,
                                                        [tmpl.id]: !prev[tmpl.id]
                                                    }));
                                                }}
                                            >
                                                {expandedBodies[tmpl.id] ? '▲ Collapse' : '▼ Expand'}
                                            </Button>
                                        </div>
                                        <div style={{
                                            maxHeight: expandedBodies[tmpl.id] ? 'none' : '60px',
                                            overflow: 'hidden',
                                            transition: 'max-height 0.3s ease-in-out'
                                        }}>
                                            <pre className="whitespace-pre-wrap font-mono text-sm text-foreground m-0 select-text">{tmpl.body}</pre>
                                        </div>
                                        {!expandedBodies[tmpl.id] && (
                                            <div className="absolute bottom-0 left-0 right-0 h-8 bg-gradient-to-t from-muted to-transparent pointer-events-none rounded-b" />
                                        )}
                                    </div>

                                    <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-4">
                                        <div className="text-sm text-muted-foreground">
                                            <strong className="text-foreground">Variables:</strong> {tmpl.variables && tmpl.variables.length > 0 ? tmpl.variables.join(', ') : 'None'}
                                        </div>
                                        <div className="flex gap-2">
                                            <Button
                                                onClick={() => togglePreview(tmpl.id)}
                                                variant="secondary"
                                                size="sm"
                                            >
                                                {activePreviews[tmpl.id] ? 'Close Preview' : 'Preview'}
                                            </Button>
                                            <Button
                                                onClick={() => openDiffViewer(tmpl)}
                                                variant="outline"
                                                size="sm"
                                            >
                                                Compare
                                            </Button>
                                            <Button
                                                onClick={() => setTestTemplate(tmpl)}
                                                variant="outline"
                                                size="sm"
                                            >
                                                Test
                                            </Button>
                                            <Button
                                                onClick={() => setControlsTemplate(tmpl)}
                                                variant="outline"
                                                size="sm"
                                            >
                                                Controls
                                            </Button>
                                            <Button
                                                onClick={() => handleSaveVersion(tmpl)}
                                                variant="outline"
                                                size="sm"
                                                disabled={savingVersion}
                                            >
                                                {savingVersion ? 'Saving...' : 'Save Version'}
                                            </Button>
                                            <Button
                                                onClick={() => handleEditTemplate(tmpl)}
                                                variant="outline"
                                                size="sm"
                                            >
                                                Edit
                                            </Button>
                                            <Button
                                                onClick={() => handleDeleteTemplate(tmpl.id)}
                                                variant="destructive"
                                                size="sm"
                                            >
                                                Delete
                                            </Button>
                                        </div>
                                    </div>

                                    {activePreviews[tmpl.id] && (
                                        <div className="mt-4 border-t border-border pt-4">
                                            <div className="space-y-2">
                                                <div className="flex items-center justify-between mb-2">
                                                    <div className="text-xs text-muted-foreground font-semibold">PREVIEW DATA (JSON)</div>
                                                    <Button
                                                        type="button"
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-6 px-2"
                                                        onClick={() => {
                                                            try {
                                                                const formatted = JSON.stringify(JSON.parse(activePreviews[tmpl.id].data), null, 2);
                                                                setActivePreviews({
                                                                    ...activePreviews,
                                                                    [tmpl.id]: { ...activePreviews[tmpl.id], data: formatted }
                                                                });
                                                            } catch {
                                                                // Invalid JSON — leave as-is
                                                            }
                                                        }}
                                                    >
                                                        Format JSON
                                                    </Button>
                                                </div>
                                                <Textarea
                                                    className="h-[120px] font-mono text-xs"
                                                    value={activePreviews[tmpl.id].data}
                                                    onChange={(e) => setActivePreviews({
                                                        ...activePreviews,
                                                        [tmpl.id]: { ...activePreviews[tmpl.id], data: e.target.value }
                                                    })}
                                                    placeholder='{"name": "Jack"}'
                                                />
                                                <div className="flex gap-2">
                                                    <Button
                                                        className="flex-1 text-xs"
                                                        onClick={() => handleRenderPreview(tmpl.id)}
                                                        disabled={activePreviews[tmpl.id].loading}
                                                    >
                                                        {activePreviews[tmpl.id].loading ? 'Rendering...' : 'Render Preview'}
                                                    </Button>
                                                    {activePreviews[tmpl.id].rendered && (
                                                        <Button
                                                            variant="outline"
                                                            className="text-xs"
                                                            onClick={() => setSlidePreview({ templateId: tmpl.id, templateName: tmpl.name, channel: tmpl.channel })}
                                                        >
                                                            View Output →
                                                        </Button>
                                                    )}
                                                </div>
                                            </div>
                                        </div>
                                    )}
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

                {/* Rendered Output Slide Panel */}
                <SlidePanel
                    open={!!slidePreview}
                    onClose={() => setSlidePreview(null)}
                    title={slidePreview ? `Rendered: ${slidePreview.templateName}` : 'Preview'}
                >
                    {slidePreview && activePreviews[slidePreview.templateId]?.rendered ? (
                        slidePreview.channel === 'email' ? (
                            <iframe
                                srcDoc={activePreviews[slidePreview.templateId].rendered}
                                sandbox=""
                                className="w-full border rounded bg-white"
                                style={{ height: 'calc(100vh - 120px)' }}
                                title="Rendered Preview"
                            />
                        ) : (
                            <div className="bg-muted min-h-[200px] p-4 rounded border border-border overflow-y-auto text-sm text-foreground whitespace-pre-wrap">
                                {activePreviews[slidePreview.templateId].rendered}
                            </div>
                        )
                    ) : (
                        <div className="flex items-center justify-center h-40 text-muted-foreground italic">
                            No rendered output yet. Click "Render Preview" first.
                        </div>
                    )}
                </SlidePanel>

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

                {/* Rollback Confirmation Dialog */}
                <ConfirmDialog
                    open={!!rollbackTarget}
                    onOpenChange={(open) => { if (!open) setRollbackTarget(null); }}
                    title="Confirm Rollback"
                    description={rollbackTarget ? `Roll back "${rollbackTarget.template.name}" to version ${rollbackTarget.version.version}? This will replace the current template content.` : ''}
                    confirmLabel="Rollback"
                    variant="destructive"
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
                                            <TableCell className="text-sm text-foreground truncate max-w-[200px]">
                                                {v.subject || '—'}
                                            </TableCell>
                                            <TableCell>
                                                <div className="flex gap-2">
                                                    <Dialog>
                                                        <DialogTrigger asChild>
                                                            <Button variant="ghost" size="sm" className="text-xs">Preview</Button>
                                                        </DialogTrigger>
                                                        <DialogContent className="max-w-xl">
                                                            <DialogHeader>
                                                                <DialogTitle>Version {v.version} Preview</DialogTitle>
                                                            </DialogHeader>
                                                            <div className="bg-muted p-4 rounded border max-h-[400px] overflow-auto">
                                                                <pre className="whitespace-pre-wrap text-sm font-mono">{v.body}</pre>
                                                            </div>
                                                        </DialogContent>
                                                    </Dialog>
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs text-foreground"
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
