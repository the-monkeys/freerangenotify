import React, { useState } from 'react';
import { whatsappTemplatesAPI } from '../../services/api';
import { useApiQuery } from '../../hooks/use-api-query';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Badge } from '../../components/ui/badge';
import { Spinner } from '../../components/ui/spinner';
import EmptyState from '../../components/EmptyState';
import { toast } from 'sonner';
import { Plus, RefreshCw, Trash2, FileText, Search } from 'lucide-react';
import type { WhatsAppMetaTemplate } from '../../types';

interface WhatsAppTemplatesProps {
    apiKey: string;
    appId: string;
}

const statusColors: Record<string, string> = {
    APPROVED: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    PENDING: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    REJECTED: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    DISABLED: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
};

const WhatsAppTemplates: React.FC<WhatsAppTemplatesProps> = ({ apiKey, appId }) => {
    const [searchName, setSearchName] = useState('');
    const [showCreate, setShowCreate] = useState(false);
    const [creating, setCreating] = useState(false);
    const [newTemplate, setNewTemplate] = useState({
        name: '',
        category: 'MARKETING',
        language: 'en_US',
        body: '',
    });

    const { data, loading, refetch } = useApiQuery(
        () => whatsappTemplatesAPI.list(apiKey),
        [apiKey],
        { enabled: !!apiKey, cacheKey: `wa-templates-${appId}` }
    );

    const templates: WhatsAppMetaTemplate[] = data?.data || [];
    const filtered = searchName
        ? templates.filter((t) => t.name.toLowerCase().includes(searchName.toLowerCase()))
        : templates;

    const handleCreate = async () => {
        if (!newTemplate.name || !newTemplate.body) {
            toast.error('Name and body are required');
            return;
        }
        setCreating(true);
        try {
            await whatsappTemplatesAPI.create(apiKey, {
                name: newTemplate.name,
                category: newTemplate.category,
                language: newTemplate.language,
                components: [
                    { type: 'BODY', text: newTemplate.body },
                ],
            });
            toast.success('Template submitted for approval');
            setShowCreate(false);
            setNewTemplate({ name: '', category: 'MARKETING', language: 'en_US', body: '' });
            refetch();
        } catch (err: any) {
            toast.error('Failed to create template: ' + (err.response?.data?.error || err.message));
        } finally {
            setCreating(false);
        }
    };

    const handleDelete = async (name: string) => {
        if (!confirm(`Delete template "${name}"? This cannot be undone.`)) return;
        try {
            await whatsappTemplatesAPI.delete(apiKey, name);
            toast.success('Template deleted');
            refetch();
        } catch (err: any) {
            toast.error('Delete failed: ' + (err.response?.data?.error || err.message));
        }
    };

    const handleSync = async (name: string) => {
        try {
            await whatsappTemplatesAPI.sync(apiKey, name);
            toast.success('Template synced from Meta');
            refetch();
        } catch (err: any) {
            toast.error('Sync failed: ' + (err.response?.data?.error || err.message));
        }
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
                    <h3 className="text-lg font-semibold">WhatsApp Templates</h3>
                    <p className="text-sm text-muted-foreground">
                        Manage Meta-approved message templates for outbound messaging outside the 24-hour window.
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

            {showCreate && (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base">Create New Template</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div className="space-y-2">
                                <Label>Template Name</Label>
                                <Input
                                    value={newTemplate.name}
                                    onChange={(e) => setNewTemplate({ ...newTemplate, name: e.target.value })}
                                    placeholder="order_confirmation"
                                />
                                <p className="text-xs text-muted-foreground">Lowercase, underscores only</p>
                            </div>
                            <div className="space-y-2">
                                <Label>Category</Label>
                                <select
                                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                                    value={newTemplate.category}
                                    onChange={(e) => setNewTemplate({ ...newTemplate, category: e.target.value })}
                                >
                                    <option value="MARKETING">Marketing</option>
                                    <option value="UTILITY">Utility</option>
                                    <option value="AUTHENTICATION">Authentication</option>
                                </select>
                            </div>
                            <div className="space-y-2">
                                <Label>Language</Label>
                                <Input
                                    value={newTemplate.language}
                                    onChange={(e) => setNewTemplate({ ...newTemplate, language: e.target.value })}
                                    placeholder="en_US"
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
                                Use {'{{1}}'}, {'{{2}}'}, etc. for variables. Template will be submitted to Meta for review.
                            </p>
                        </div>
                        <div className="flex gap-2">
                            <Button onClick={handleCreate} disabled={creating}>
                                {creating ? <Spinner className="h-4 w-4 mr-1" /> : null}
                                Submit for Approval
                            </Button>
                            <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
                        </div>
                    </CardContent>
                </Card>
            )}

            <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                    className="pl-9"
                    placeholder="Search templates..."
                    value={searchName}
                    onChange={(e) => setSearchName(e.target.value)}
                />
            </div>

            {filtered.length === 0 ? (
                <EmptyState
                    icon={<FileText className="h-10 w-10" />}
                    title="No WhatsApp templates"
                    description={templates.length === 0 ? 'Create your first Meta WhatsApp template to start sending messages outside the 24-hour window.' : 'No templates match your search.'}
                />
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                    {filtered.map((tpl) => (
                        <Card key={tpl.id} className="relative">
                            <CardHeader className="pb-2">
                                <div className="flex items-start justify-between">
                                    <div className="space-y-1 min-w-0">
                                        <CardTitle className="text-sm font-medium truncate">{tpl.name}</CardTitle>
                                        <div className="flex gap-2">
                                            <Badge variant="outline" className="text-xs">{tpl.language}</Badge>
                                            <Badge className={`text-xs ${statusColors[tpl.status] || statusColors.DISABLED}`}>
                                                {tpl.status}
                                            </Badge>
                                        </div>
                                    </div>
                                    <Badge variant="secondary" className="text-xs shrink-0">{tpl.category}</Badge>
                                </div>
                            </CardHeader>
                            <CardContent className="pt-2">
                                {tpl.components?.find((c: any) => c.type === 'BODY') && (
                                    <p className="text-xs text-muted-foreground line-clamp-3">
                                        {tpl.components.find((c: any) => c.type === 'BODY')?.text}
                                    </p>
                                )}
                                <div className="flex gap-1 mt-3">
                                    <Button variant="ghost" size="sm" onClick={() => handleSync(tpl.name)}>
                                        <RefreshCw className="h-3 w-3 mr-1" /> Sync
                                    </Button>
                                    <Button variant="ghost" size="sm" className="text-destructive" onClick={() => handleDelete(tpl.name)}>
                                        <Trash2 className="h-3 w-3 mr-1" /> Delete
                                    </Button>
                                </div>
                            </CardContent>
                        </Card>
                    ))}
                </div>
            )}
        </div>
    );
};

export default WhatsAppTemplates;
