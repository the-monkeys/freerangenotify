import React, { useState, useMemo } from 'react';
import { twilioTemplatesAPI } from '../../services/api';
import { useApiQuery } from '../../hooks/use-api-query';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Badge } from '../../components/ui/badge';
import { Spinner } from '../../components/ui/spinner';
import EmptyState from '../../components/EmptyState';
import WhatsAppPreview from '../../components/whatsapp/WhatsAppPreview';
import { toast } from 'sonner';
import {
    Plus,
    RefreshCw,
    Trash2,
    FileText,
    Search,
    Send,
    Eye,
    CheckCircle2,
    ChevronDown,
    ChevronUp,
    Copy,
    Pencil,
    CopyPlus,
} from 'lucide-react';
import type { TwilioContentTemplate } from '../../types';

/** Extract {{N}} variable placeholders from a template body string. */
function extractVariableKeys(body: string): string[] {
    const matches = body.match(/\{\{\s*(\w+)\s*\}\}/g);
    if (!matches) return [];
    const keys = [...new Set(matches.map(m => m.replace(/[{}\s]/g, '')))];
    const numeric = keys.filter(k => /^\d+$/.test(k)).sort((a, b) => Number(a) - Number(b));
    const named = keys.filter(k => !/^\d+$/.test(k));
    return [...numeric, ...named];
}

interface TwilioWhatsAppTemplatesProps {
    apiKey: string;
    appId: string;
}

const approvalStatusColors: Record<string, string> = {
    approved: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    rejected: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    unsubmitted: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
    received: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
};

function getApprovalStatus(tpl: TwilioContentTemplate): string {
    return tpl.approval_requests?.status || 'unsubmitted';
}

function getApprovalCategory(tpl: TwilioContentTemplate): string {
    return tpl.approval_requests?.category || '';
}

function getBodyText(tpl: TwilioContentTemplate): string {
    if (!tpl.types) return '';
    // Check common Twilio content types
    for (const key of ['twilio/text', 'twilio/quick-reply', 'twilio/media', 'twilio/call-to-action', 'twilio/card', 'twilio/list-picker']) {
        const typeObj = tpl.types[key];
        if (typeObj?.body) return typeObj.body;
    }
    return JSON.stringify(tpl.types, null, 2);
}

const TwilioWhatsAppTemplates: React.FC<TwilioWhatsAppTemplatesProps> = ({ apiKey, appId }) => {
    const [searchTerm, setSearchTerm] = useState('');
    const [showCreate, setShowCreate] = useState(false);
    const [creating, setCreating] = useState(false);
    const [expandedSid, setExpandedSid] = useState<string | null>(null);
    const [approvalForm, setApprovalForm] = useState<{ sid: string; name: string; category: string } | null>(null);
    const [submittingApproval, setSubmittingApproval] = useState(false);
    const [previewData, setPreviewData] = useState<{ sid: string; rendered: any } | null>(null);
    const [previewVars, setPreviewVars] = useState<Record<string, string>>({});
    const [previewLoading, setPreviewLoading] = useState(false);

    const [newTemplate, setNewTemplate] = useState({
        friendly_name: '',
        language: 'en',
        body: '',
    });
    // Example values for variable placeholders — required by Meta/WhatsApp for approval
    const [newTemplateVarExamples, setNewTemplateVarExamples] = useState<Record<string, string>>({});

    // Auto-extracted variable keys from the create/edit body
    const newTemplateVarKeys = useMemo(() => extractVariableKeys(newTemplate.body), [newTemplate.body]);

    // ── Edit mode for unsubmitted templates ──
    const [editingSid, setEditingSid] = useState<string | null>(null);
    const [editForm, setEditForm] = useState({ friendly_name: '', body: '' });
    const [editVarExamples, setEditVarExamples] = useState<Record<string, string>>({});
    const [saving, setSaving] = useState(false);
    const editVarKeys = useMemo(() => extractVariableKeys(editForm.body), [editForm.body]);

    const { data: rawData, loading, refetch } = useApiQuery(
        () => twilioTemplatesAPI.list(apiKey),
        [apiKey],
        { enabled: !!apiKey, cacheKey: `twilio-templates-${appId}` }
    );

    const templates: TwilioContentTemplate[] = rawData?.contents || [];
    const filtered = searchTerm
        ? templates.filter((t) => t.friendly_name?.toLowerCase().includes(searchTerm.toLowerCase()))
        : templates;

    const handleCreate = async () => {
        if (!newTemplate.friendly_name || !newTemplate.body) {
            toast.error('Name and body text are required');
            return;
        }
        // Validate that all detected variables have example values (Meta requires these)
        const missingExamples = newTemplateVarKeys.filter(k => !newTemplateVarExamples[k]?.trim());
        if (missingExamples.length > 0) {
            toast.error(`Provide example values for: ${missingExamples.map(k => `{{${k}}}`).join(', ')}`);
            return;
        }
        setCreating(true);
        try {
            const variables: Record<string, string> | undefined =
                newTemplateVarKeys.length > 0
                    ? Object.fromEntries(newTemplateVarKeys.map(k => [k, newTemplateVarExamples[k]?.trim() || '']))
                    : undefined;
            await twilioTemplatesAPI.create(apiKey, {
                friendly_name: newTemplate.friendly_name,
                language: newTemplate.language,
                types: {
                    'twilio/text': { body: newTemplate.body },
                },
                variables,
            });
            toast.success('Template created successfully');
            setShowCreate(false);
            setNewTemplate({ friendly_name: '', language: 'en', body: '' });
            setNewTemplateVarExamples({});
            refetch();
        } catch (err: any) {
            toast.error('Failed to create template: ' + (err.response?.data?.error || err.message));
        } finally {
            setCreating(false);
        }
    };

    // ── Edit handler (unsubmitted templates only) ──
    const startEdit = (tpl: TwilioContentTemplate) => {
        setEditingSid(tpl.sid);
        setEditForm({
            friendly_name: tpl.friendly_name,
            body: getBodyText(tpl),
        });
        setEditVarExamples(tpl.variables ? { ...tpl.variables } : {});
    };

    const handleSaveEdit = async () => {
        if (!editingSid) return;
        const missingExamples = editVarKeys.filter(k => !editVarExamples[k]?.trim());
        if (missingExamples.length > 0) {
            toast.error(`Provide example values for: ${missingExamples.map(k => `{{${k}}}`).join(', ')}`);
            return;
        }
        setSaving(true);
        try {
            const variables: Record<string, string> | undefined =
                editVarKeys.length > 0
                    ? Object.fromEntries(editVarKeys.map(k => [k, editVarExamples[k]?.trim() || '']))
                    : undefined;
            await twilioTemplatesAPI.update(apiKey, editingSid, {
                friendly_name: editForm.friendly_name,
                types: { 'twilio/text': { body: editForm.body } },
                variables,
            });
            toast.success('Template updated');
            setEditingSid(null);
            refetch();
        } catch (err: any) {
            toast.error('Update failed: ' + (err.response?.data?.error || err.message));
        } finally {
            setSaving(false);
        }
    };

    // ── Clone handler (for rejected templates — can't edit after submission) ──
    const handleClone = (tpl: TwilioContentTemplate) => {
        setNewTemplate({
            friendly_name: tpl.friendly_name + ' (copy)',
            language: tpl.language,
            body: getBodyText(tpl),
        });
        setNewTemplateVarExamples(tpl.variables ? { ...tpl.variables } : {});
        setShowCreate(true);
        // Scroll to create form
        window.scrollTo({ top: 0, behavior: 'smooth' });
    };

    const handleDelete = async (sid: string, name: string) => {
        if (!confirm(`Delete template "${name}"? This cannot be undone.`)) return;
        try {
            await twilioTemplatesAPI.delete(apiKey, sid);
            toast.success('Template deleted');
            if (expandedSid === sid) setExpandedSid(null);
            refetch();
        } catch (err: any) {
            toast.error('Delete failed: ' + (err.response?.data?.error || err.message));
        }
    };

    const handleSync = async (sid: string) => {
        try {
            await twilioTemplatesAPI.sync(apiKey, sid);
            toast.success('Template status synced');
            refetch();
        } catch (err: any) {
            toast.error('Sync failed: ' + (err.response?.data?.error || err.message));
        }
    };

    const handleSubmitApproval = async () => {
        if (!approvalForm) return;
        setSubmittingApproval(true);
        try {
            await twilioTemplatesAPI.submitApproval(apiKey, approvalForm.sid, {
                name: approvalForm.name,
                category: approvalForm.category,
            });
            toast.success('Template submitted for WhatsApp approval');
            setApprovalForm(null);
            refetch();
        } catch (err: any) {
            toast.error('Approval submission failed: ' + (err.response?.data?.error || err.message));
        } finally {
            setSubmittingApproval(false);
        }
    };

    const handlePreview = async (tpl: TwilioContentTemplate) => {
        setPreviewLoading(true);
        try {
            const result = await twilioTemplatesAPI.preview(apiKey, tpl.sid, previewVars);
            setPreviewData({ sid: tpl.sid, rendered: result });
        } catch (err: any) {
            toast.error('Preview failed: ' + (err.response?.data?.error || err.message));
        } finally {
            setPreviewLoading(false);
        }
    };

    const copySid = (sid: string) => {
        navigator.clipboard.writeText(sid);
        toast.success('Content SID copied');
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center py-16">
                <Spinner className="h-6 w-6" />
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-lg font-semibold">Twilio WhatsApp Templates</h3>
                    <p className="text-sm text-muted-foreground">
                        Create and manage Twilio Content Templates for WhatsApp messaging. Templates must be approved before use.
                    </p>
                </div>
                <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => refetch()}>
                        <RefreshCw className="h-4 w-4 mr-1" /> Refresh
                    </Button>
                    <Button size="sm" onClick={() => setShowCreate(!showCreate)}>
                        <Plus className="h-4 w-4 mr-1" /> New Template
                    </Button>
                </div>
            </div>

            {/* Create Form */}
            {showCreate && (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base">Create New Twilio Content Template</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label>Friendly Name</Label>
                                <Input
                                    value={newTemplate.friendly_name}
                                    onChange={(e) => setNewTemplate({ ...newTemplate, friendly_name: e.target.value })}
                                    placeholder="e.g. Order Confirmation"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>Language</Label>
                                <Input
                                    value={newTemplate.language}
                                    onChange={(e) => setNewTemplate({ ...newTemplate, language: e.target.value })}
                                    placeholder="en"
                                />
                            </div>
                        </div>
                        <div className="space-y-2">
                            <Label>Body Text</Label>
                            <textarea
                                className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                                value={newTemplate.body}
                                onChange={(e) => setNewTemplate({ ...newTemplate, body: e.target.value })}
                                placeholder="Hello {{1}}, your order {{2}} has been confirmed."
                            />
                            <p className="text-xs text-muted-foreground">
                                Use {'{{1}}'}, {'{{2}}'}, etc. for variables. After creating, submit for WhatsApp approval.
                            </p>
                        </div>
                        {newTemplateVarKeys.length > 0 && (
                            <div className="space-y-3 p-3 rounded-md border border-dashed border-yellow-300 dark:border-yellow-700 bg-yellow-50/50 dark:bg-yellow-950/30">
                                <p className="text-xs font-medium text-yellow-800 dark:text-yellow-200">
                                    WhatsApp requires example values for each variable to approve the template.
                                </p>
                                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                    {newTemplateVarKeys.map(k => (
                                        <div key={k} className="space-y-1">
                                            <Label className="text-xs">
                                                <code className="bg-muted px-1 rounded mr-1">{`{{${k}}}`}</code> example
                                            </Label>
                                            <Input
                                                value={newTemplateVarExamples[k] || ''}
                                                onChange={(e) => setNewTemplateVarExamples(prev => ({ ...prev, [k]: e.target.value }))}
                                                placeholder={`e.g. ${k === '1' ? 'John' : k === '2' ? '12345' : 'sample_value'}`}
                                                className="h-8 text-sm"
                                            />
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}
                        <div className="flex gap-2">
                            <Button onClick={handleCreate} disabled={creating}>
                                {creating ? <Spinner className="h-4 w-4 mr-1" /> : null}
                                Create Template
                            </Button>
                            <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
                        </div>
                    </CardContent>
                </Card>
            )}

            {/* Approval Submission Dialog */}
            {approvalForm && (
                <Card className="border-yellow-200 dark:border-yellow-800">
                    <CardHeader>
                        <CardTitle className="text-base flex items-center gap-2">
                            <Send className="h-4 w-4" />
                            Submit for WhatsApp Approval
                        </CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <p className="text-sm text-muted-foreground">
                            Submitting template <strong>{approvalForm.sid}</strong> to WhatsApp for approval. This is required before you can send messages outside the 24-hour window.
                        </p>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label>Template Name (for WhatsApp)</Label>
                                <Input
                                    value={approvalForm.name}
                                    onChange={(e) => setApprovalForm({ ...approvalForm, name: e.target.value })}
                                    placeholder="lowercase_underscores_only"
                                />
                                <p className="text-xs text-muted-foreground">Lowercase, alphanumeric and underscores only</p>
                            </div>
                            <div className="space-y-2">
                                <Label>Category</Label>
                                <select
                                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                                    value={approvalForm.category}
                                    onChange={(e) => setApprovalForm({ ...approvalForm, category: e.target.value })}
                                >
                                    <option value="UTILITY">Utility</option>
                                    <option value="MARKETING">Marketing</option>
                                    <option value="AUTHENTICATION">Authentication</option>
                                </select>
                            </div>
                        </div>
                        <div className="flex gap-2">
                            <Button onClick={handleSubmitApproval} disabled={submittingApproval}>
                                {submittingApproval ? <Spinner className="h-4 w-4 mr-1" /> : <CheckCircle2 className="h-4 w-4 mr-1" />}
                                Submit for Approval
                            </Button>
                            <Button variant="outline" onClick={() => setApprovalForm(null)}>Cancel</Button>
                        </div>
                    </CardContent>
                </Card>
            )}

            {/* Search */}
            <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                    className="pl-9"
                    placeholder="Search templates by name..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                />
            </div>

            {/* Template List */}
            {filtered.length === 0 ? (
                <EmptyState
                    icon={<FileText className="h-10 w-10" />}
                    title="No Twilio WhatsApp templates"
                    description={templates.length === 0
                        ? 'Create your first Twilio Content Template to start sending WhatsApp messages.'
                        : 'No templates match your search.'}
                />
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {filtered.map((tpl) => {
                        const status = getApprovalStatus(tpl);
                        const category = getApprovalCategory(tpl);
                        const body = getBodyText(tpl);
                        const isExpanded = expandedSid === tpl.sid;

                        return (
                            <Card key={tpl.sid} className="relative flex flex-col">
                                <CardHeader className="pb-2">
                                    <div className="flex items-start justify-between gap-2">
                                        <div className="space-y-1 min-w-0 flex-1">
                                            <CardTitle className="text-sm font-medium truncate">{tpl.friendly_name}</CardTitle>
                                            <div className="flex flex-wrap gap-1.5">
                                                <Badge variant="outline" className="text-xs">{tpl.language}</Badge>
                                                <Badge className={`text-xs ${approvalStatusColors[status] || approvalStatusColors.unsubmitted}`}>
                                                    {status}
                                                </Badge>
                                                {category && (
                                                    <Badge variant="secondary" className="text-xs">{category}</Badge>
                                                )}
                                            </div>
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="h-6 w-6 p-0 shrink-0"
                                            onClick={() => setExpandedSid(isExpanded ? null : tpl.sid)}
                                        >
                                            {isExpanded ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                                        </Button>
                                    </div>
                                </CardHeader>
                                <CardContent className="pt-2 flex-1 flex flex-col">
                                    {/* Inline edit form (unsubmitted only) */}
                                    {editingSid === tpl.sid ? (
                                        <div className="space-y-3">
                                            <div className="space-y-1">
                                                <Label className="text-xs">Friendly Name</Label>
                                                <Input
                                                    value={editForm.friendly_name}
                                                    onChange={(e) => setEditForm(f => ({ ...f, friendly_name: e.target.value }))}
                                                    className="h-8 text-sm"
                                                />
                                            </div>
                                            <div className="space-y-1">
                                                <Label className="text-xs">Body Text</Label>
                                                <textarea
                                                    className="flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-xs"
                                                    value={editForm.body}
                                                    onChange={(e) => setEditForm(f => ({ ...f, body: e.target.value }))}
                                                />
                                            </div>
                                            {editVarKeys.length > 0 && (
                                                <div className="space-y-2 p-2 rounded border border-dashed border-yellow-300 dark:border-yellow-700 bg-yellow-50/50 dark:bg-yellow-950/30">
                                                    <p className="text-[11px] font-medium text-yellow-800 dark:text-yellow-200">Example values (required for approval)</p>
                                                    {editVarKeys.map(k => (
                                                        <div key={k} className="flex items-center gap-2">
                                                            <code className="text-[11px] bg-muted px-1 rounded shrink-0">{`{{${k}}}`}</code>
                                                            <Input
                                                                value={editVarExamples[k] || ''}
                                                                onChange={(e) => setEditVarExamples(prev => ({ ...prev, [k]: e.target.value }))}
                                                                placeholder="example value"
                                                                className="h-7 text-xs"
                                                            />
                                                        </div>
                                                    ))}
                                                </div>
                                            )}
                                            <div className="flex gap-2">
                                                <Button size="sm" className="h-7 text-xs" onClick={handleSaveEdit} disabled={saving}>
                                                    {saving ? <Spinner className="h-3 w-3 mr-1" /> : null}
                                                    Save
                                                </Button>
                                                <Button variant="outline" size="sm" className="h-7 text-xs" onClick={() => setEditingSid(null)}>
                                                    Cancel
                                                </Button>
                                            </div>
                                        </div>
                                    ) : (
                                        <>
                                            <p className="text-xs text-muted-foreground line-clamp-3 flex-1">
                                                {body}
                                            </p>

                                            {/* Expanded details */}
                                            {isExpanded && (
                                                <div className="mt-3 space-y-3 border-t pt-3">
                                                    <div className="flex items-center gap-1 text-xs text-muted-foreground">
                                                        <span className="font-medium">SID:</span>
                                                        <code className="bg-muted px-1 rounded text-[11px]">{tpl.sid}</code>
                                                        <Button variant="ghost" size="sm" className="h-5 w-5 p-0" onClick={() => copySid(tpl.sid)}>
                                                            <Copy className="h-3 w-3" />
                                                        </Button>
                                                    </div>
                                                    {tpl.variables && Object.keys(tpl.variables).length > 0 && (
                                                        <div className="text-xs">
                                                            <span className="font-medium text-muted-foreground">Variables:</span>
                                                            <div className="mt-1 space-y-1">
                                                                {Object.entries(tpl.variables).map(([key, desc]) => (
                                                                    <div key={key} className="flex gap-2">
                                                                        <code className="bg-muted px-1 rounded text-[11px]">{`{{${key}}}`}</code>
                                                                        <span className="text-muted-foreground">{desc}</span>
                                                                    </div>
                                                                ))}
                                                            </div>
                                                        </div>
                                                    )}
                                                    {tpl.approval_requests?.rejection_reason && (
                                                        <div className="text-xs p-2 rounded bg-red-50 dark:bg-red-950 text-red-700 dark:text-red-300">
                                                            <span className="font-medium">Rejection reason:</span> {tpl.approval_requests.rejection_reason}
                                                        </div>
                                                    )}

                                                    {/* Preview section */}
                                                    {previewData?.sid === tpl.sid && (
                                                        <div className="space-y-2 flex flex-col items-center">
                                                            <span className="text-xs font-medium text-muted-foreground self-start">WhatsApp Preview:</span>
                                                            <WhatsAppPreview
                                                                template={tpl}
                                                                variables={previewVars}
                                                                header={tpl.friendly_name}
                                                                compact
                                                            />
                                                        </div>
                                                    )}

                                                    {/* Variable inputs for preview */}
                                                    {tpl.variables && Object.keys(tpl.variables).length > 0 && (
                                                        <div className="space-y-2">
                                                            <span className="text-xs font-medium text-muted-foreground">Test variables:</span>
                                                            {Object.entries(tpl.variables).map(([key]) => (
                                                                <Input
                                                                    key={key}
                                                                    className="h-7 text-xs"
                                                                    placeholder={`{{${key}}}`}
                                                                    value={previewVars[key] || ''}
                                                                    onChange={(e) => setPreviewVars({ ...previewVars, [key]: e.target.value })}
                                                                />
                                                            ))}
                                                        </div>
                                                    )}
                                                </div>
                                            )}

                                            {/* Actions */}
                                            <div className="flex flex-wrap gap-1 mt-3 pt-2 border-t">
                                                {status === 'unsubmitted' && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-7"
                                                        onClick={() => setApprovalForm({
                                                            sid: tpl.sid,
                                                            name: tpl.friendly_name.toLowerCase().replace(/[^a-z0-9]+/g, '_'),
                                                            category: 'UTILITY',
                                                        })}
                                                    >
                                                        <Send className="h-3 w-3 mr-1" /> Submit Approval
                                                    </Button>
                                                )}
                                                {status === 'unsubmitted' && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-7"
                                                        onClick={() => startEdit(tpl)}
                                                    >
                                                        <Pencil className="h-3 w-3 mr-1" /> Edit
                                                    </Button>
                                                )}
                                                {status === 'rejected' && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-7"
                                                        onClick={() => handleClone(tpl)}
                                                    >
                                                        <CopyPlus className="h-3 w-3 mr-1" /> Clone &amp; Fix
                                                    </Button>
                                                )}
                                                <Button variant="ghost" size="sm" className="text-xs h-7" onClick={() => handleSync(tpl.sid)}>
                                                    <RefreshCw className="h-3 w-3 mr-1" /> Sync
                                                </Button>
                                                {isExpanded && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-7"
                                                        onClick={() => handlePreview(tpl)}
                                                        disabled={previewLoading}
                                                    >
                                                        {previewLoading ? <Spinner className="h-3 w-3 mr-1" /> : <Eye className="h-3 w-3 mr-1" />}
                                                        Preview
                                                    </Button>
                                                )}
                                                {status === 'approved' && (
                                                    <Button
                                                        variant="ghost"
                                                        size="sm"
                                                        className="text-xs h-7"
                                                        onClick={() => copySid(tpl.sid)}
                                                    >
                                                        <Copy className="h-3 w-3 mr-1" /> Copy SID
                                                    </Button>
                                                )}
                                                <Button
                                                    variant="ghost"
                                                    size="sm"
                                                    className="text-xs h-7 text-destructive"
                                                    onClick={() => handleDelete(tpl.sid, tpl.friendly_name)}
                                                >
                                                    <Trash2 className="h-3 w-3 mr-1" /> Delete
                                                </Button>
                                            </div>
                                        </>
                                    )}
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            )}

            {/* Usage Instructions */}
            {templates.some(t => getApprovalStatus(t) === 'approved') && (
                <Card className="bg-muted/40">
                    <CardContent className="pt-4">
                        <h4 className="text-sm font-semibold mb-2">How to use approved templates</h4>
                        <p className="text-xs text-muted-foreground mb-2">
                            When sending a WhatsApp notification via the API, include the template&apos;s Content SID in the notification data:
                        </p>
                        <pre className="text-xs bg-background p-3 rounded-md overflow-x-auto">
                            {`{
  "channel": "whatsapp",
  "content": {
    "data": {
      "content_sid": "<template_sid>",
      "content_variables": "{\\"1\\":\\"value1\\",\\"2\\":\\"value2\\"}"
    }
  }
}`}
                        </pre>
                    </CardContent>
                </Card>
            )}
        </div>
    );
};

export default TwilioWhatsAppTemplates;
