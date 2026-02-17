import React, { useEffect, useState } from 'react';
import { templatesAPI } from '../services/api';import { type Template, type CreateTemplateRequest, TemplateChannel } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Textarea } from './ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Badge } from './ui/badge';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from './ui/dialog';
import { toast } from 'sonner';

interface AppTemplatesProps {
    appId: string;
    apiKey: string;
    webhooks?: Record<string, string>;
}

const AppTemplates: React.FC<AppTemplatesProps> = ({ appId, apiKey, webhooks }) => {
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [formData, setFormData] = useState<CreateTemplateRequest>({
        app_id: appId,
        name: '',
        channel: TemplateChannel.EMAIL,
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

    // Edit dialog state
    const [editOpen, setEditOpen] = useState(false);
    const [editForm, setEditForm] = useState<Partial<CreateTemplateRequest> & { id?: string }>({});
    const [editVarInput, setEditVarInput] = useState('');

    useEffect(() => {
        if (apiKey) {
            fetchTemplates();
        }
    }, [apiKey]);

    const fetchTemplates = async () => {
        setLoading(true);
        try {
            const data = await templatesAPI.list(apiKey);
            setTemplates(data || []);
        } catch (error) {
            console.error('Failed to fetch templates:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleCreateTemplate = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            await templatesAPI.create(apiKey, { ...formData, app_id: appId });
            setShowAddForm(false);
            setFormData({
                app_id: appId,
                name: '',
                channel: TemplateChannel.EMAIL,
                webhook_target: '',
                subject: '',
                body: '',
                description: '',
                variables: []
            });
            fetchTemplates();
            toast.success('Template created successfully!');
        } catch (error) {
            console.error('Failed to create template:', error);
            toast.error('Failed to create template');
        }
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

    const openEditDialog = (tmpl: Template) => {
        setEditForm({
            id: tmpl.id,
            app_id: tmpl.app_id,
            name: tmpl.name,
            channel: tmpl.channel as TemplateChannel,
            webhook_target: tmpl.webhook_target || '',
            subject: tmpl.subject || '',
            body: tmpl.body,
            description: tmpl.description || '',
            variables: tmpl.variables || []
        });
        setEditVarInput('');
        setEditOpen(true);
    };

    const handleUpdateTemplate = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!editForm.id) return;

        try {
            await templatesAPI.update(apiKey, editForm.id, {
                description: editForm.description || '',
                webhook_target: editForm.webhook_target || '',
                subject: editForm.subject || '',
                body: editForm.body || '',
                variables: editForm.variables || []
            });
            setEditOpen(false);
            setEditForm({});
            fetchTemplates();
            toast.success('Template updated successfully!');
        } catch (error) {
            console.error('Failed to update template:', error);
            toast.error('Failed to update template');
        }
    };

    const handleAddEditVariable = () => {
        const vars = editForm.variables || [];
        if (editVarInput && !vars.includes(editVarInput)) {
            setEditForm({ ...editForm, variables: [...vars, editVarInput] });
            setEditVarInput('');
        }
    };

    const togglePreview = (tmplId: string) => {
        if (activePreviews[tmplId]) {
            const newPreviews = { ...activePreviews };
            delete newPreviews[tmplId];
            setActivePreviews(newPreviews);
        } else {
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { data: '{}', rendered: '', loading: false }
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
        } catch (error) {
            console.error('Failed to render preview:', error);
            toast.error('Failed to render preview');
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { ...preview, loading: false }
            });
        }
    };

    if (loading) return <div className="flex justify-center py-4">Loading templates...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle>Notification Templates</CardTitle>
                    <Button
                        onClick={() => setShowAddForm(!showAddForm)}
                    >
                        {showAddForm ? 'Cancel' : 'Create Template'}
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {showAddForm && (
                    <form onSubmit={handleCreateTemplate} className="mb-8 bg-gray-50 p-6 rounded border border-gray-200 space-y-4">
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
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="channel">Channel</Label>
                                <Select
                                    value={formData.channel}
                                    onValueChange={(value) => setFormData({ ...formData, channel: value as any })}
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
                                    value={formData.webhook_target || ''}
                                    onValueChange={(value) => setFormData({ ...formData, webhook_target: value })}
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder="Default (Application Webhook URL)" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="">Default (Application Webhook URL)</SelectItem>
                                        {Object.keys(webhooks).map(name => (
                                            <SelectItem key={name} value={name}>{name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-gray-500">
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
                            <Textarea
                                id="body"
                                className="min-h-40 font-mono"
                                value={formData.body}
                                onChange={(e) => {
                                    // Simple regex to auto-detect variables like {{.var_name}}
                                    const newBody = e.target.value;
                                    const regex = /{{\s*\.?(\w+)\s*}}/g;
                                    const matches = new Set<string>();
                                    let match;
                                    while ((match = regex.exec(newBody)) !== null) {
                                        if (match[1]) matches.add(match[1]);
                                    }
                                    // Combine custom added vars with auto-detected ones
                                    const currentVars = new Set(formData.variables || []);
                                    for (const m of matches) currentVars.add(m);

                                    setFormData({
                                        ...formData,
                                        body: newBody,
                                        variables: Array.from(currentVars)
                                    });
                                }}
                                required
                                placeholder="Hello {{.name}}, welcome!"
                            />
                            <p className="text-xs text-gray-500">
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
                            <Button type="submit">Create Template</Button>
                        </div>
                    </form>
                )}

                {!templates || templates.length === 0 ? (
                    <p className="text-gray-500 text-center py-8">No templates found.</p>
                ) : (
                    <div className="space-y-6">
                        {templates.map((tmpl) => (
                            <Card key={tmpl.id} className="bg-white border-gray-200">
                                <CardContent className="pt-6">
                                    <div className="flex justify-between items-start mb-4">
                                        <div>
                                            <h4 className="text-lg font-semibold text-blue-600 mb-1">{tmpl.name}</h4>
                                            <p className="text-sm text-gray-500">{tmpl.description || 'No description'}</p>
                                        </div>
                                        <Badge variant="outline" className="text-xs uppercase bg-gray-50 text-blue-600 border-blue-600">
                                            {tmpl.channel}
                                        </Badge>
                                    </div>

                                    {tmpl.channel === 'webhook' && (
                                        <div className="mb-2 text-sm font-semibold text-blue-600">
                                            Target: <span className="text-gray-900">{tmpl.webhook_target || 'Default'}</span>
                                        </div>
                                    )}

                                    <div
                                        className="mb-4 bg-gray-50 p-4 rounded border border-dashed border-gray-200 cursor-pointer relative group transition-colors hover:bg-gray-100"
                                        onClick={() => {
                                            setExpandedBodies(prev => ({
                                                ...prev,
                                                [tmpl.id]: !prev[tmpl.id]
                                            }));
                                        }}
                                        title="Click to expand/collapse"
                                    >
                                        <div className="flex justify-between items-center mb-2">
                                            <div className="text-xs text-gray-500 font-semibold">TEMPLATE BODY</div>
                                            <div className="text-[10px] text-blue-600 font-bold opacity-0 group-hover:opacity-100 transition-opacity">
                                                {expandedBodies[tmpl.id] ? 'COLLAPSE' : 'EXPAND'}
                                            </div>
                                        </div>
                                        <div style={{
                                            maxHeight: expandedBodies[tmpl.id] ? 'none' : '60px',
                                            overflow: 'hidden',
                                            transition: 'max-height 0.3s ease-in-out'
                                        }}>
                                            <pre className="whitespace-pre-wrap font-mono text-sm text-gray-900 m-0">{tmpl.body}</pre>
                                        </div>
                                        {!expandedBodies[tmpl.id] && (
                                            <div className="absolute bottom-0 left-0 right-0 h-8 bg-linear-to-t from-gray-50 to-transparent pointer-events-none rounded-b" />
                                        )}
                                    </div>

                                    <div className="flex justify-between items-center mb-4">
                                        <div className="text-sm text-gray-500">
                                            <strong className="text-gray-900">Variables:</strong> {tmpl.variables && tmpl.variables.length > 0 ? tmpl.variables.join(', ') : 'None'}
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
                                                onClick={() => openEditDialog(tmpl)}
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
                                        <div className="mt-4 border-t border-gray-200 pt-4">
                                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                                <div className="space-y-2">
                                                    <div className="text-xs text-gray-500 font-semibold mb-2">PREVIEW DATA (JSON)</div>
                                                    <Textarea
                                                        className="h-25 font-mono text-xs"
                                                        value={activePreviews[tmpl.id].data}
                                                        onChange={(e) => setActivePreviews({
                                                            ...activePreviews,
                                                            [tmpl.id]: { ...activePreviews[tmpl.id], data: e.target.value }
                                                        })}
                                                        placeholder='{"name": "Jack"}'
                                                    />
                                                    <Button
                                                        className="w-full text-xs"
                                                        onClick={() => handleRenderPreview(tmpl.id)}
                                                        disabled={activePreviews[tmpl.id].loading}
                                                    >
                                                        {activePreviews[tmpl.id].loading ? 'Rendering...' : 'Render Preview'}
                                                    </Button>
                                                </div>
                                                <div className="space-y-2">
                                                    <div className="text-xs text-gray-500 font-semibold mb-2">RENDERED OUTPUT</div>
                                                    <div className="bg-gray-50 h-25 p-3 rounded border border-gray-200 overflow-y-auto text-sm text-gray-900">
                                                        {activePreviews[tmpl.id].rendered || <span className="text-gray-400 italic">Click Render to see output...</span>}
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    )}
                                </CardContent>
                            </Card>
                        ))}
                    </div>
                )}
            </CardContent>

            <Dialog open={editOpen} onOpenChange={setEditOpen}>
                <DialogContent className="sm:max-w-2xl">
                    <DialogHeader>
                        <DialogTitle>Edit Template</DialogTitle>
                        <DialogDescription>
                            Update template content and variables. Name and channel are read-only.
                        </DialogDescription>
                    </DialogHeader>
                    <form onSubmit={handleUpdateTemplate} className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="editTemplateName">Template Name</Label>
                                <Input
                                    id="editTemplateName"
                                    type="text"
                                    value={editForm.name || ''}
                                    disabled
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="editChannel">Channel</Label>
                                <Select
                                    value={editForm.channel || ''}
                                    onValueChange={(value) => setEditForm({ ...editForm, channel: value as TemplateChannel })}
                                    disabled
                                >
                                    <SelectTrigger className='w-full'>
                                        <SelectValue placeholder="Select a channel" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value={TemplateChannel.EMAIL}>Email</SelectItem>
                                        <SelectItem value={TemplateChannel.SMS}>SMS</SelectItem>
                                        <SelectItem value={TemplateChannel.WEBHOOK}>Webhook</SelectItem>
                                        <SelectItem value={TemplateChannel.PUSH}>Push</SelectItem>
                                        <SelectItem value={TemplateChannel.IN_APP}>In-App</SelectItem>
                                        <SelectItem value={TemplateChannel.SSE}>SSE (Server-Sent Events)</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>

                        {editForm.channel === 'webhook' && webhooks && Object.keys(webhooks).length > 0 && (
                            <div className="space-y-2">
                                <Label htmlFor="editWebhookTarget">Webhook Target</Label>
                                <Select
                                    value={editForm.webhook_target || ''}
                                    onValueChange={(value) => setEditForm({ ...editForm, webhook_target: value })}
                                >
                                    <SelectTrigger className='w-full'>
                                        <SelectValue placeholder="Default (Application Webhook URL)" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="">Default (Application Webhook URL)</SelectItem>
                                        {Object.keys(webhooks).map(name => (
                                            <SelectItem key={name} value={name}>{name}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-gray-500">
                                    Select a specific named webhook endpoint for this template.
                                </p>
                            </div>
                        )}

                        <div className="space-y-2">
                            <Label htmlFor="editSubject">Subject (for Email)</Label>
                            <Input
                                id="editSubject"
                                type="text"
                                value={editForm.subject || ''}
                                onChange={(e) => setEditForm({ ...editForm, subject: e.target.value })}
                                placeholder="Email subject"
                            />
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="editBody">Body / Content</Label>
                            <Textarea
                                id="editBody"
                                className="min-h-40 font-mono"
                                value={editForm.body || ''}
                                onChange={(e) => {
                                    const newBody = e.target.value;
                                    const regex = /{{\s*\.?([\w]+)\s*}}/g;
                                    const matches = new Set<string>();
                                    let match;
                                    while ((match = regex.exec(newBody)) !== null) {
                                        if (match[1]) matches.add(match[1]);
                                    }
                                    const currentVars = new Set(editForm.variables || []);
                                    for (const m of matches) currentVars.add(m);

                                    setEditForm({
                                        ...editForm,
                                        body: newBody,
                                        variables: Array.from(currentVars)
                                    });
                                }}
                                required
                                placeholder="Hello {{.name}}, welcome!"
                            />
                            <p className="text-xs text-gray-500">
                                Use <code>{'{{.variable_name}}'}</code> syntax. Detected variables will enter the list below automatically.
                            </p>
                        </div>

                        <div className="space-y-2">
                            <Label>Variables (Must be declared to pass validation)</Label>
                            <div className="flex gap-2">
                                <Input
                                    type="text"
                                    value={editVarInput}
                                    onChange={(e) => setEditVarInput(e.target.value)}
                                    placeholder="name"
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            e.preventDefault();
                                            handleAddEditVariable();
                                        }
                                    }}
                                />
                                <Button type="button" variant="secondary" onClick={handleAddEditVariable}>Add</Button>
                            </div>
                            <div className="mt-2 flex gap-2 flex-wrap">
                                {(editForm.variables || []).map(v => (
                                    <Badge key={v} variant="outline" className="text-sm">
                                        {v}
                                        <button
                                            type="button"
                                            onClick={() => setEditForm({
                                                ...editForm,
                                                variables: (editForm.variables || []).filter(x => x !== v)
                                            })}
                                            className="ml-2 text-red-600 hover:text-red-700 font-bold"
                                        >
                                            &times;
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="editDescription">Description</Label>
                            <Input
                                id="editDescription"
                                type="text"
                                value={editForm.description || ''}
                                onChange={(e) => setEditForm({ ...editForm, description: e.target.value })}
                                placeholder="Optional description"
                            />
                        </div>

                        <DialogFooter>
                            <Button type="button" variant="outline" onClick={() => setEditOpen(false)}>
                                Cancel
                            </Button>
                            <Button type="submit">Save Changes</Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </Card>
    );
};

export default AppTemplates;
