import React, { useState } from 'react';
import { workflowsAPI, applicationsAPI, topicsAPI } from '../../services/api';
import type {
    WorkflowSchedule,
    CreateScheduleRequest,
    UpdateScheduleRequest,
    ScheduleTargetType,
    Application,
    Workflow,
} from '../../types';
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
import { Badge } from '../../components/ui/badge';
import { Timer, Plus, MoreHorizontal, Pencil, Trash2, Loader2, ExternalLink } from 'lucide-react';
import { Link } from 'react-router-dom';
import { timeAgo } from '../../lib/utils';
import { toast } from 'sonner';
import { TimezonePicker } from '../../components/TimezonePicker';

const getBrowserTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone;

interface SchedulesListProps {
    apiKey?: string;
    embedded?: boolean;
}

const PAGE_SIZE = 15;

const SchedulesList: React.FC<SchedulesListProps> = ({ apiKey: propApiKey, embedded }) => {
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
        () => workflowsAPI.listSchedules(apiKey!, PAGE_SIZE, offset),
        [apiKey, offset],
        { enabled: !!apiKey }
    );

    const { data: workflowsData } = useApiQuery(
        () => workflowsAPI.list(apiKey!, 100, 0),
        [apiKey],
        { enabled: !!apiKey }
    );
    const workflows: Workflow[] = workflowsData?.workflows ?? [];

    const { data: topicsData } = useApiQuery(
        () => topicsAPI.list(apiKey!, 100, 0),
        [apiKey],
        { enabled: !!apiKey }
    );
    const topics = topicsData?.topics ?? [];

    // Editor state
    const [showEditor, setShowEditor] = useState(false);
    const [editingSchedule, setEditingSchedule] = useState<WorkflowSchedule | null>(null);
    const [saving, setSaving] = useState(false);
    const [formName, setFormName] = useState('');
    const [formWorkflowTriggerId, setFormWorkflowTriggerId] = useState('');
    const [formCron, setFormCron] = useState('');
    const [formTimezone, setFormTimezone] = useState(getBrowserTimezone);
    const [formTargetType, setFormTargetType] = useState<ScheduleTargetType>('all');
    const [formTopicId, setFormTopicId] = useState('');
    const [formPayload, setFormPayload] = useState('');
    const [formStatus, setFormStatus] = useState<'active' | 'inactive'>('active');

    // Delete
    const [deleteTarget, setDeleteTarget] = useState<WorkflowSchedule | null>(null);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const apps = await applicationsAPI.list();
            const app = apps.find((a: Application) => a.app_id === appId);
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
        setEditingSchedule(null);
        setFormName('');
        setFormWorkflowTriggerId('');
        setFormCron('0 9 * * 1');
        setFormTimezone(getBrowserTimezone());
        setFormTargetType('all');
        setFormTopicId('');
        setFormPayload('{}');
        setFormStatus('active');
        setShowEditor(true);
    };

    const openEdit = (schedule: WorkflowSchedule) => {
        setEditingSchedule(schedule);
        setFormName(schedule.name);
        setFormWorkflowTriggerId(schedule.workflow_trigger_id);
        setFormCron(schedule.cron);
        setFormTimezone(schedule.timezone || getBrowserTimezone());
        setFormTargetType(schedule.target_type);
        setFormTopicId(schedule.topic_id || '');
        setFormPayload(
            schedule.payload ? JSON.stringify(schedule.payload, null, 2) : '{}'
        );
        setFormStatus(schedule.status);
        setShowEditor(true);
    };

    const parsePayload = (): Record<string, unknown> | undefined => {
        if (!formPayload.trim()) return undefined;
        try {
            const parsed = JSON.parse(formPayload);
            return typeof parsed === 'object' && parsed !== null ? parsed : undefined;
        } catch {
            return undefined;
        }
    };

    const handleSave = async () => {
        if (!apiKey || !formName.trim() || !formWorkflowTriggerId || !formCron.trim()) {
            toast.error('Name, Workflow, and Cron are required');
            return;
        }
        const payload = parsePayload();
        if (formPayload.trim() && payload === undefined) {
            toast.error('Payload must be valid JSON');
            return;
        }
        setSaving(true);
        try {
            if (editingSchedule) {
                const updatePayload: UpdateScheduleRequest = {
                    name: formName.trim(),
                    workflow_trigger_id: formWorkflowTriggerId,
                    cron: formCron.trim(),
                    timezone: formTimezone || undefined,
                    target_type: formTargetType,
                    topic_id: formTargetType === 'topic' && formTopicId ? formTopicId : undefined,
                    payload: payload,
                    status: formStatus,
                };
                await workflowsAPI.updateSchedule(apiKey, editingSchedule.id, updatePayload);
                toast.success('Schedule updated');
            } else {
                const createPayload: CreateScheduleRequest = {
                    name: formName.trim(),
                    workflow_trigger_id: formWorkflowTriggerId,
                    cron: formCron.trim(),
                    target_type: formTargetType,
                    topic_id: formTargetType === 'topic' && formTopicId ? formTopicId : undefined,
                    payload: payload,
                };
                await workflowsAPI.createSchedule(apiKey, createPayload);
                toast.success('Schedule created');
            }
            setShowEditor(false);
            refetch();
        } catch {
            toast.error('Failed to save schedule');
        } finally {
            setSaving(false);
        }
    };

    const handleDelete = async () => {
        if (!apiKey || !deleteTarget) return;
        try {
            await workflowsAPI.deleteSchedule(apiKey, deleteTarget.id);
            toast.success('Schedule deleted');
            setDeleteTarget(null);
            refetch();
        } catch {
            toast.error('Failed to delete schedule');
        }
    };

    const schedules: WorkflowSchedule[] = data?.schedules || [];
    const total: number = data?.total || 0;

    if (!apiKey && !embedded) {
        return (
            <div className="p-6 max-w-6xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">Schedules</h1>
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
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                        <Timer className="h-5 w-5 text-muted-foreground" />
                        <h1 className="text-2xl font-semibold text-foreground">Schedules</h1>
                    </div>
                    <Button onClick={openCreate}>
                        <Plus className="h-4 w-4 mr-2" />
                        New Schedule
                    </Button>
                </div>
            )}

            {embedded && (
                <div className="flex items-center justify-between">
                    <p className="text-sm text-muted-foreground">
                        {total} schedule{total !== 1 ? 's' : ''}
                    </p>
                    <Button size="sm" onClick={openCreate}>
                        <Plus className="h-4 w-4 mr-1.5" />
                        New Schedule
                    </Button>
                </div>
            )}

            {/* Table */}
            {loading ? (
                <SkeletonTable rows={5} columns={5} />
            ) : schedules.length === 0 ? (
                <EmptyState
                    title="No schedules"
                    description="Create schedules to run workflows on cron (e.g. daily digest)"
                    action={{ label: 'New Schedule', onClick: openCreate }}
                />
            ) : (
                <>
                    <div className="border border-border rounded-lg overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Name</TableHead>
                                    <TableHead className="hidden md:table-cell">Workflow</TableHead>
                                    <TableHead className="hidden md:table-cell">Cron</TableHead>
                                    <TableHead className="hidden lg:table-cell">Target</TableHead>
                                    <TableHead className="hidden lg:table-cell">Last Run</TableHead>
                                    <TableHead className="w-10" />
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {schedules.map((s) => (
                                    <TableRow key={s.id}>
                                        <TableCell>
                                            <div className="flex items-center gap-2">
                                                <span className="font-medium">{s.name}</span>
                                                <Badge
                                                    variant={s.status === 'active' ? 'default' : 'secondary'}
                                                    className="text-xs"
                                                >
                                                    {s.status}
                                                </Badge>
                                            </div>
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell font-mono text-xs text-muted-foreground">
                                            {s.workflow_trigger_id}
                                        </TableCell>
                                        <TableCell className="hidden md:table-cell font-mono text-xs">
                                            {s.cron}
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell text-sm">
                                            {s.target_type === 'all' ? (
                                                <span>All users</span>
                                            ) : (
                                                <span>Topic: {s.topic_id ? s.topic_id.slice(0, 8) + '…' : '—'}</span>
                                            )}
                                        </TableCell>
                                        <TableCell className="hidden lg:table-cell text-xs text-muted-foreground">
                                            {s.last_run_at ? (
                                                <span title={s.last_run_at}>{timeAgo(s.last_run_at)}</span>
                                            ) : (
                                                'Never'
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button variant="ghost" size="sm">
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    {workflows.find(w => w.trigger_id === s.workflow_trigger_id) && (
                                                        <DropdownMenuItem asChild>
                                                            <Link
                                                                to={`/workflows/executions?workflow_id=${workflows.find(w => w.trigger_id === s.workflow_trigger_id)!.id}`}
                                                            >
                                                                <ExternalLink className="h-3.5 w-3.5 mr-2" />
                                                                View workflow executions
                                                            </Link>
                                                        </DropdownMenuItem>
                                                    )}
                                                    <DropdownMenuItem onClick={() => openEdit(s)}>
                                                        <Pencil className="h-3.5 w-3.5 mr-2" />
                                                        Edit
                                                    </DropdownMenuItem>
                                                    <DropdownMenuItem
                                                        className="text-destructive"
                                                        onClick={() => setDeleteTarget(s)}
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

            {/* Schedule Editor Panel */}
            <SlidePanel
                open={showEditor}
                onClose={() => setShowEditor(false)}
                title={editingSchedule ? 'Edit Schedule' : 'New Schedule'}
            >
                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>Name <span className="text-destructive">*</span></Label>
                        <Input
                            value={formName}
                            onChange={(e) => setFormName(e.target.value)}
                            placeholder="e.g., Weekly digest"
                        />
                    </div>
                    {workflows.length > 0 ? (
                        <div className="space-y-2">
                            <Label>Workflow <span className="text-destructive">*</span></Label>
                            <Select
                                value={formWorkflowTriggerId || 'none'}
                                onValueChange={(val: string) =>
                                    setFormWorkflowTriggerId(val === 'none' ? '' : val)
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder="Select workflow" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">— Select —</SelectItem>
                                    {workflows.map((w) => (
                                        <SelectItem key={w.id} value={w.trigger_id}>
                                            {w.name} ({w.trigger_id})
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    ) : (
                        <div className="space-y-2">
                            <Label>Workflow Trigger ID <span className="text-destructive">*</span></Label>
                            <Input
                                value={formWorkflowTriggerId}
                                onChange={(e) => setFormWorkflowTriggerId(e.target.value)}
                                placeholder="e.g., weekly_digest"
                            />
                            <p className="text-xs text-muted-foreground">
                                Create a workflow first, then use its trigger_id here
                            </p>
                        </div>
                    )}
                    <div className="space-y-2">
                        <Label>Timezone</Label>
                        <TimezonePicker
                            value={formTimezone}
                            onChange={setFormTimezone}
                            placeholder="Search timezone..."
                        />
                        <p className="text-xs text-muted-foreground">
                            Cron runs at this timezone (e.g. 9am Monday = 9am in this zone)
                        </p>
                    </div>
                    <div className="space-y-2">
                        <Label>Cron <span className="text-destructive">*</span></Label>
                        <Input
                            value={formCron}
                            onChange={(e) => setFormCron(e.target.value)}
                            placeholder="0 9 * * 1"
                            className="font-mono"
                        />
                        <div className="rounded border border-border bg-muted/40 p-3 text-xs text-muted-foreground space-y-2">
                            <p className="font-medium text-foreground">How does the schedule work?</p>
                            <p>Enter 5 values separated by spaces, in this order:</p>
                            <ul className="list-disc pl-4 space-y-1">
                                <li><strong>Minute</strong> (0–59): When in the hour. Use 0 for the top of the hour, 30 for half past.</li>
                                <li><strong>Hour</strong> (0–23): 9 = 9 AM, 21 = 9 PM. Use 24-hour format.</li>
                                <li><strong>Day of month</strong> (1–31): Use * for every day, or a number like 15 for the 15th.</li>
                                <li><strong>Month</strong> (1–12): Use * for every month, or a number like 6 for June.</li>
                                <li><strong>Day of week</strong> (0–7): 0 and 7 = Sunday, 1 = Monday, 5 = Friday. Use * for every day.</li>
                            </ul>
                            <p className="pt-1">Examples: <code className="bg-muted px-1 rounded">0 9 * * 1</code> = Mondays at 9:00 AM · <code className="bg-muted px-1 rounded">0 0 * * *</code> = Daily at midnight</p>
                        </div>
                    </div>
                    <div className="space-y-2">
                        <Label>Target</Label>
                        <Select
                            value={formTargetType}
                            onValueChange={(val: string) =>
                                setFormTargetType(val as ScheduleTargetType)
                            }
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="all">All users</SelectItem>
                                <SelectItem value="topic">Topic subscribers</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    {formTargetType === 'topic' && (
                        <div className="space-y-2">
                            <Label>Topic</Label>
                            <Select
                                value={formTopicId || 'none'}
                                onValueChange={(val: string) =>
                                    setFormTopicId(val === 'none' ? '' : val)
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue placeholder="Select topic" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="none">— Select —</SelectItem>
                                    {topics.map((t) => (
                                        <SelectItem key={t.id} value={t.id}>
                                            {t.name} ({t.key})
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    )}
                    <div className="space-y-2">
                        <Label>Payload (optional JSON)</Label>
                        <Textarea
                            value={formPayload}
                            onChange={(e) => setFormPayload(e.target.value)}
                            placeholder='{"key": "value"}'
                            rows={4}
                            className="font-mono text-sm"
                        />
                        <p className="text-xs text-muted-foreground">
                            Additional data passed to the workflow (optional)
                        </p>
                    </div>
                    {editingSchedule && (
                        <div className="space-y-2">
                            <Label>Status</Label>
                            <Select
                                value={formStatus}
                                onValueChange={(val: string) =>
                                    setFormStatus(val as 'active' | 'inactive')
                                }
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="active">Active</SelectItem>
                                    <SelectItem value="inactive">Inactive</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    )}
                    <div className="flex items-center gap-2 pt-2">
                        <Button onClick={handleSave} disabled={saving}>
                            {saving && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                            {editingSchedule ? 'Update' : 'Create'}
                        </Button>
                        <Button variant="outline" onClick={() => setShowEditor(false)}>
                            Cancel
                        </Button>
                    </div>
                </div>
            </SlidePanel>

            <ConfirmDialog
                open={!!deleteTarget}
                onOpenChange={(open) => !open && setDeleteTarget(null)}
                title="Delete schedule"
                description={
                    deleteTarget
                        ? `Are you sure you want to delete "${deleteTarget.name}"?`
                        : ''
                }
                onConfirm={handleDelete}
            />
        </div>
    );
};

export default SchedulesList;
