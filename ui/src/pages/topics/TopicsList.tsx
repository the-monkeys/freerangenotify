import React, { useState, useCallback } from 'react';
import { topicsAPI, usersAPI, applicationsAPI, workflowsAPI } from '../../services/api';
import type { Topic, TopicSubscription, CreateTopicRequest, UpdateTopicRequest, User, Application, Workflow } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import ResourcePicker from '../../components/ResourcePicker';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import { Pagination } from '../../components/Pagination';
import { SlidePanel } from '../../components/ui/slide-panel';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Textarea } from '../../components/ui/textarea';
import { Badge } from '../../components/ui/badge';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '../../components/ui/table';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/ui/select';
import { Tag, Plus, MoreHorizontal, Pencil, Trash2, Users, Loader2, X } from 'lucide-react';
import { timeAgo } from '../../lib/utils';
import { toast } from 'sonner';

interface TopicsListProps {
    apiKey?: string;
    embedded?: boolean;
}

const PAGE_SIZE = 15;

const TopicsList: React.FC<TopicsListProps> = ({ apiKey: propApiKey, embedded }) => {
    const [ownApiKey, setOwnApiKey] = useState<string | null>(
        propApiKey || localStorage.getItem('last_api_key')
    );
    const [selectedAppId, setSelectedAppId] = useState<string | null>(
        localStorage.getItem('last_app_id')
    );
    const apiKey = propApiKey || ownApiKey;

    const [page, setPage] = useState(1);
    const offset = (page - 1) * PAGE_SIZE;

    const { data, loading, refetch } = useApiQuery(
        () => topicsAPI.list(apiKey!, PAGE_SIZE, offset),
        [apiKey, offset],
        { 
            enabled: !!apiKey,
            cacheKey: `topics-list-${apiKey}-${offset}`
        }

    );

    const { data: workflowsData } = useApiQuery(
        () => workflowsAPI.list(apiKey!, 100, 0),
        [apiKey],
        { enabled: !!apiKey }
    );
    const workflows: Workflow[] = workflowsData?.workflows ?? [];

    // Topic editor state
    const [showEditor, setShowEditor] = useState(false);
    const [editingTopic, setEditingTopic] = useState<Topic | null>(null);
    const [saving, setSaving] = useState(false);
    const [formName, setFormName] = useState('');
    const [formKey, setFormKey] = useState('');
    const [formDesc, setFormDesc] = useState('');
    const [formOnSubscribeTriggerId, setFormOnSubscribeTriggerId] = useState('');

    // Subscribers panel state
    const [subscriberTopic, setSubscriberTopic] = useState<Topic | null>(null);
    const [subscribers, setSubscribers] = useState<TopicSubscription[]>([]);
    const [loadingSubs, setLoadingSubs] = useState(false);
    const [addUserId, setAddUserId] = useState<string | null>(null);
    const [addingSub, setAddingSub] = useState(false);

    // Delete
    const [deleteTarget, setDeleteTarget] = useState<Topic | null>(null);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const app = await applicationsAPI.get(appId);
            if (app) {
                setSelectedAppId(app.app_id);
                setOwnApiKey(app.api_key);
                localStorage.setItem('last_app_id', app.app_id);
                localStorage.setItem('last_api_key', app.api_key);
            }
        } catch {
            toast.error('Failed to load application');
        }
    };

    const openCreate = () => {
        setEditingTopic(null);
        setFormName('');
        setFormKey('');
        setFormDesc('');
        setFormOnSubscribeTriggerId('');
        setShowEditor(true);
    };

    const openEdit = (topic: Topic) => {
        setEditingTopic(topic);
        setFormName(topic.name);
        setFormKey(topic.key);
        setFormDesc(topic.description || '');
        setFormOnSubscribeTriggerId(topic.on_subscribe_trigger_id || '');
        setShowEditor(true);
    };

    const handleSave = async () => {
        if (!apiKey || !formName.trim() || !formKey.trim()) {
            toast.error('Name and Key are required');
            return;
        }
        setSaving(true);
        try {
            if (editingTopic) {
                const updatePayload: UpdateTopicRequest = {
                    name: formName.trim(),
                    key: formKey.trim(),
                    description: formDesc.trim() || undefined,
                    on_subscribe_trigger_id: formOnSubscribeTriggerId || undefined,
                };
                await topicsAPI.update(apiKey, editingTopic.id, updatePayload);
                toast.success('Topic updated');
            } else {
                const payload: CreateTopicRequest = {
                    name: formName.trim(),
                    key: formKey.trim(),
                    description: formDesc.trim() || undefined,
                    on_subscribe_trigger_id: formOnSubscribeTriggerId || undefined,
                };
                await topicsAPI.create(apiKey, payload);
                toast.success('Topic created');
            }
            setShowEditor(false);
            refetch();
        } catch {
            toast.error('Failed to save topic');
        } finally {
            setSaving(false);
        }
    };

    const handleDelete = async () => {
        if (!apiKey || !deleteTarget) return;
        try {
            await topicsAPI.delete(apiKey, deleteTarget.id);
            toast.success('Topic deleted');
            setDeleteTarget(null);
            refetch();
        } catch {
            toast.error('Failed to delete topic');
        }
    };

    // Subscribers
    const loadSubscribers = useCallback(async (topic: Topic) => {
        if (!apiKey) return;
        setSubscriberTopic(topic);
        setLoadingSubs(true);
        try {
            const res = await topicsAPI.getSubscribers(apiKey, topic.id, 100, 0);
            setSubscribers(res.subscribers || []);
        } catch {
            toast.error('Failed to load subscribers');
        } finally {
            setLoadingSubs(false);
        }
    }, [apiKey]);

    const handleAddSubscriber = async () => {
        if (!apiKey || !subscriberTopic || !addUserId) return;
        setAddingSub(true);
        try {
            await topicsAPI.addSubscribers(apiKey, subscriberTopic.id, { user_ids: [addUserId] });
            toast.success('Subscriber added');
            setAddUserId(null);
            loadSubscribers(subscriberTopic);
            refetch(); // update subscriber_count
        } catch {
            toast.error('Failed to add subscriber');
        } finally {
            setAddingSub(false);
        }
    };

    const handleRemoveSubscriber = async (userId: string) => {
        if (!apiKey || !subscriberTopic) return;
        try {
            await topicsAPI.removeSubscribers(apiKey, subscriberTopic.id, { user_ids: [userId] });
            toast.success('Subscriber removed');
            loadSubscribers(subscriberTopic);
            refetch();
        } catch {
            toast.error('Failed to remove subscriber');
        }
    };

    const topics: Topic[] = data?.topics || [];
    const total: number = data?.total || 0;

    if (!apiKey && !embedded) {
        return (
            <div className="p-6 max-w-6xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">Topics</h1>
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                    />
                </div>
            </div>
        );
    }

    return (
        <div className={embedded ? 'space-y-4' : 'p-6 max-w-6xl mx-auto space-y-6'}>
            {/* Header */}
            {!embedded && (
                <div className="space-y-4">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <Tag className="h-5 w-5 text-muted-foreground" />
                            <h1 className="text-2xl font-semibold text-foreground">Topics</h1>
                        </div>
                        <Button onClick={openCreate}>
                            <Plus className="h-4 w-4 mr-2" />
                            New Topic
                        </Button>
                    </div>

                    {/* App picker (standalone only) */}
                    <div className="max-w-xs">
                        <ResourcePicker<Application>
                            label="Application"
                            value={selectedAppId}
                            onChange={handleAppSelect}
                            fetcher={async () => applicationsAPI.list()}
                            labelKey="app_name"
                            valueKey="app_id"
                            placeholder="Select an application..."
                        />
                    </div>
                </div>
            )}

            {embedded && (
                <div className="flex items-center justify-between">
                    <p className="text-sm text-muted-foreground">{total} topic{total !== 1 ? 's' : ''}</p>
                    <Button size="sm" onClick={openCreate}>
                        <Plus className="h-4 w-4 mr-1.5" />
                        New Topic
                    </Button>
                </div>
            )}

            {/* Table */}
            {loading ? (
                <SkeletonTable rows={5} columns={5} />
            ) : topics.length === 0 ? (
                <EmptyState
                    title="No topics"
                    description="Create topics to organize notifications by category"
                    action={{ label: 'New Topic', onClick: openCreate }}
                />
            ) : (
                <>
                    <div className="border border-border rounded-lg overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Name</TableHead>
                                    <TableHead className="hidden md:table-cell">Key</TableHead>
                                    <TableHead className="hidden md:table-cell">Subscribers</TableHead>
                                    <TableHead className="hidden lg:table-cell">Updated</TableHead>
                                    <TableHead className="w-10" />
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {topics.map((topic) => (
                                    <TableRow key={topic.id}>
                                        <TableCell>
                                            <div>
                                                <span className="font-medium">{topic.name}</span>
                                                {topic.description && (
                                                    <p className="text-xs text-muted-foreground truncate max-w-[200px]">
                                                        {topic.description}
                                                    </p>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell font-mono text-xs text-muted-foreground">
                                            {topic.key}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell">
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="gap-1.5"
                                                onClick={() => loadSubscribers(topic)}
                                            >
                                                <Users className="h-3.5 w-3.5" />
                                                {topic.subscriber_count ?? 0}
                                            </Button>
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell text-xs text-muted-foreground">
                                            {timeAgo(topic.updated_at)}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="sm">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    <DropdownMenuItem onClick={() => openEdit(topic)}>
                                                        <Pencil className="h-3.5 w-3.5 mr-2" />
                                                        Edit
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem onClick={() => loadSubscribers(topic)}>
                                                        <Users className="h-3.5 w-3.5 mr-2" />
                                                        Subscribers
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        className="text-destructive"
                                                        onClick={() => setDeleteTarget(topic)}
                                                    >
                                                        <Trash2 className="h-3.5 w-3.5 mr-2" />
                                                        Delete
                                                    </DropdownMenuItem>
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>

                    {total > PAGE_SIZE && (
                        <Pagination
                            currentPage={page}
                            totalItems={total}
                            pageSize={PAGE_SIZE}
                            onPageChange={setPage}
                        />
                    )}
                </>
            )}

            {/* Topic Editor Panel */}
            <SlidePanel
                open={showEditor}
                onClose={() => setShowEditor(false)}
                title={editingTopic ? 'Edit Topic' : 'New Topic'}
            >
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Name <span className="text-destructive">*</span></Label>
                        <Input value={formName} onChange={(e) => setFormName(e.target.value)} placeholder="e.g., Marketing Updates" />
                    </div>
                    <div className="space-y-2">
                        <Label>Key <span className="text-destructive">*</span></Label>
                        <Input value={formKey} onChange={(e) => setFormKey(e.target.value)} placeholder="e.g., marketing" className="font-mono" />
                        <p className="text-xs text-muted-foreground">Unique identifier for this topic</p>
                    </div>
                    <div className="space-y-2">
                        <Label>Description</Label>
                        <Textarea value={formDesc} onChange={(e) => setFormDesc(e.target.value)} placeholder="Optional topic description" rows={3} />
                    </div>
                    {workflows.length > 0 && (
                        <div className="space-y-2">
                            <Label>On Subscribe Workflow (optional)</Label>
                            <Select
                                value={formOnSubscribeTriggerId || 'none'}
                                onValueChange={(val: string) => setFormOnSubscribeTriggerId(val === 'none' ? '' : val)}
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder="None" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">None</SelectItem>
                                    {workflows.map((w) => (
                                        <SelectItem key={w.id} value={w.trigger_id}>{w.name} ({w.trigger_id})</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">Trigger this workflow for each new subscriber when they are added</p>
                        </div>
                    )}
                    <div className="flex items-center gap-2 pt-2">
                        <Button onClick={handleSave} disabled={saving}>
                            {saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                            {editingTopic ? 'Update' : 'Create'}
                        </Button>
                        <Button variant="outline" onClick={() => setShowEditor(false)}>Cancel</Button>
                    </div>
                </div>
            </SlidePanel>

            {/* Subscribers Panel */}
            <SlidePanel
                open={!!subscriberTopic}
                onClose={() => setSubscriberTopic(null)}
                title={`Subscribers — ${subscriberTopic?.name || ''}`}
            >
                <div className="space-y-4">
                    {/* Add subscriber */}
                    <div className="flex items-end gap-2">
                        <div className="flex-1">
                            <ResourcePicker<User>
                                label="Add User"
                                value={addUserId}
                                onChange={setAddUserId}
                                fetcher={async () => {
                                    const res = await usersAPI.list(apiKey!, 1, 100);
                                    return res.users || [];
                                }}
                                labelKey="email"
                                valueKey="user_id"
                                placeholder="Search users..."
                            />
                        </div>
                        <Button
                            onClick={handleAddSubscriber}
                            disabled={!addUserId || addingSub}
                            size="sm"
                        >
                            {addingSub ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Add'}
                        </Button>
                    </div>

                    {/* Subscriber list */}
                    {loadingSubs ? (
                        <div className="flex items-center justify-center h-20">
                            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                        </div>
                    ) : subscribers.length === 0 ? (
                        <p className="text-sm text-muted-foreground py-4">No subscribers yet</p>
                    ) : (
                        <div className="space-y-1">
                            {subscribers.map((sub) => (
                                <div
                                    key={sub.id}
                                    className="flex items-center justify-between px-3 py-2 rounded-md hover:bg-muted/50"
                                >
                                    <div>
                                        <span className="font-mono text-xs">{sub.user_id}</span>
                                        <p className="text-xs text-muted-foreground">
                                            since {timeAgo(sub.created_at)}
                                        </p>
                                    </div>
                                    <Button
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => handleRemoveSubscriber(sub.user_id)}
                                    >
                                        <X className="h-3.5 w-3.5" />
                                    </Button>
                                </div>
                            ))}
                        </div>
                    )}

                    <Badge variant="secondary" className="mt-2">
                        {subscribers.length} subscriber{subscribers.length !== 1 ? 's' : ''}
                    </Badge>
                </div>
            </SlidePanel>

            {/* Delete Confirm */}
            <ConfirmDialog
                open={!!deleteTarget}
                onOpenChange={(open) => !open && setDeleteTarget(null)}
                onConfirm={handleDelete}
                title="Delete Topic"
                description={deleteTarget ? `Are you sure you want to delete "${deleteTarget.name}"? All subscriptions will be removed.` : ''}
                confirmLabel="Delete"
                variant="destructive"
            />
        </div>
    );
};

export default TopicsList;
