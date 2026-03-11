import React, { useState, useMemo, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowsAPI, applicationsAPI } from '../../services/api';
import type { Workflow, Application } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { useDebounce } from '../../hooks/use-debounce';
import ResourcePicker from '../../components/ResourcePicker';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '../../components/ui/select';
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
import { MoreHorizontal, Plus, Workflow as WorkflowIcon } from 'lucide-react';
import { toast } from 'sonner';
import { timeAgo } from '../../lib/utils';

const statusBadgeVariant: Record<string, string> = {
    active: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    draft: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    inactive: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
};

interface WorkflowsListProps {
    apiKey?: string;
    embedded?: boolean;
}

const WorkflowsList: React.FC<WorkflowsListProps> = ({ apiKey: propApiKey, embedded }) => {
    const navigate = useNavigate();

    // App context (used when not embedded)
    const [selectedAppId, setSelectedAppId] = useState<string | null>(
        localStorage.getItem('last_app_id')
    );
    const [ownApiKey, setOwnApiKey] = useState<string | null>(
        localStorage.getItem('last_api_key')
    );
    const apiKey = propApiKey || ownApiKey;

    // Filters
    const [statusFilter, setStatusFilter] = useState('all');
    const [search, setSearch] = useState('');
    const debouncedSearch = useDebounce(search, 150); // Faster live search feel

    // Data
    const { data, loading, refetch } = useApiQuery(
        () => workflowsAPI.list(apiKey!, 100, 0),
        [apiKey],
        { enabled: !!apiKey }
    );

    // Delete
    const [deleteTarget, setDeleteTarget] = useState<Workflow | null>(null);
    const [deleting, setDeleting] = useState(false);

    // Auto-select first app if none selected and not embedded
    useEffect(() => {
        if (!embedded && !apiKey) {
            applicationsAPI.list().then(apps => {
                if (apps && apps.length > 0) {
                    handleAppSelect(apps[0].app_id);
                }
            }).catch(() => { });
        }
    }, [embedded, apiKey]);

    // Filter workflows
    const workflows = useMemo(() => {
        let items = data?.workflows || [];
        if (statusFilter !== 'all') {
            items = items.filter(w => w.status === statusFilter);
        }
        if (debouncedSearch) {
            const q = debouncedSearch.toLowerCase();
            items = items.filter(w => w.name.toLowerCase().includes(q));
        }
        return items;
    }, [data, statusFilter, debouncedSearch]);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return; // Prevent deselection
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

    const handleDelete = async () => {
        if (!deleteTarget || !apiKey) return;
        setDeleting(true);
        try {
            await workflowsAPI.delete(apiKey, deleteTarget.id);
            toast.success('Workflow deleted');
            setDeleteTarget(null);
            refetch();
        } catch {
            toast.error('Failed to delete workflow');
        } finally {
            setDeleting(false);
        }
    };

    const handleToggleStatus = async (w: Workflow) => {
        if (!apiKey) return;
        const newStatus = w.status === 'active' ? 'inactive' : 'active';
        try {
            await workflowsAPI.update(apiKey, w.id, { status: newStatus });
            toast.success(`Workflow ${newStatus === 'active' ? 'activated' : 'deactivated'}`);
            refetch();
        } catch {
            toast.error('Failed to update workflow status');
        }
    };

    const handleDuplicate = async (w: Workflow) => {
        if (!apiKey) return;
        try {
            await workflowsAPI.create(apiKey, {
                name: `${w.name} (copy)`,
                description: w.description,
                trigger_id: `${w.trigger_id}_copy`,
                steps: w.steps.map(({ id, ...rest }) => rest),
            });
            toast.success('Workflow duplicated');
            refetch();
        } catch {
            toast.error('Failed to duplicate workflow');
        }
    };

    // Standalone mode without apiKey — show app picker
    if (!apiKey && !embedded) {
        return (
            <div className="p-6 max-w-6xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">Workflows</h1>
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                        hint="Workflows are scoped to an application"
                    />
                </div>
                <EmptyState
                    title="Select an application"
                    description="Choose an app above to view its workflows"
                    icon={<WorkflowIcon className="h-12 w-12" />}
                    action={{ label: 'Go to Applications', onClick: () => navigate('/apps') }}
                />
            </div>
        );
    }

    const total = data?.total ?? data?.workflows?.length ?? 0;
    return (
        <div className={embedded ? 'space-y-4' : 'p-6 max-w-6xl mx-auto space-y-6'}>
            {/* Header */}
            {!embedded && (
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-2xl font-semibold text-foreground">Workflows</h1>
                        <p className="text-sm text-muted-foreground mt-1">
                            Multi-step notification flows
                        </p>
                    </div>
                    <Button onClick={() => navigate('/workflows/new')}>
                        <Plus className="h-4 w-4 mr-2" />
                        New Workflow
                    </Button>
                </div>
            )}

            {embedded && (
                <div className="flex items-center justify-between">
                    <p className="text-sm text-muted-foreground">
                        {total} workflow{total !== 1 ? 's' : ''}
                    </p>
                    <Button size="sm" onClick={() => navigate('/workflows/new')}>
                        <Plus className="h-4 w-4 mr-1.5" />
                        New Workflow
                    </Button>
                </div>
            )}

            {/* App picker (standalone only) */}
            {!embedded && (
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                        hint="Workflows are scoped to an application"
                    />
                </div>
            )}

            {/* Content */}
            {apiKey && (
                <>
                    {/* Filters */}
                    <div className="flex items-center gap-3">
                        <div className="w-40">
                            <Select value={statusFilter} onValueChange={setStatusFilter}>
                                <SelectTrigger className="h-9">
                                    <SelectValue placeholder="Status" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="all">All Statuses</SelectItem>
                                    <SelectItem value="draft">Draft</SelectItem>
                                    <SelectItem value="active">Active</SelectItem>
                                    <SelectItem value="inactive">Inactive</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <Input
                            placeholder="Search workflows..."
                            value={search}
                            onChange={(e) => setSearch(e.target.value)}
                            className="max-w-xs h-9"
                        />
                    </div>

                    {/* Loading */}
                    {loading && <SkeletonTable rows={5} columns={6} />}

                    {/* Empty */}
                    {!loading && workflows.length === 0 && (
                        <EmptyState
                            title="No workflows yet"
                            description="Create your first multi-step notification flow"
                            icon={<WorkflowIcon className="h-12 w-12" />}
                            action={{ label: 'Create Workflow', onClick: () => navigate('/workflows/new') }}
                        />
                    )}

                    {/* Table */}
                    {!loading && workflows.length > 0 && (
                        <div className="border border-border rounded-lg overflow-hidden">
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Name</TableHead>
                                        <TableHead className="hidden md:table-cell">Trigger ID</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead className="hidden md:table-cell">Steps</TableHead>
                                        <TableHead className="hidden md:table-cell">Version</TableHead>
                                        <TableHead className="hidden lg:table-cell">Updated</TableHead>
                                        <TableHead className="w-10" />
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {workflows.map(w => (
                                        <TableRow
                                            key={w.id}
                                            className="cursor-pointer hover:bg-muted/50"
                                            onClick={() => navigate(`/workflows/${w.id}`)}
                                        >
                                            <TableCell className="font-medium">{w.name}</TableCell>
                                            <TableCell className="hidden md:table-cell">
                                                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                    {w.trigger_id}
                                                </code>
                                            </TableCell>
                                            <TableCell onClick={(e) => e.stopPropagation()}>
                                                <div className="flex items-center gap-2">
                                                    <button
                                                        type="button"
                                                        role="switch"
                                                        aria-checked={w.status === 'active'}
                                                        onClick={() => handleToggleStatus(w)}
                                                        className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center justify-center rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 ${w.status === 'active' ? 'bg-primary' : 'bg-input'}`}
                                                    >
                                                        <span
                                                            className={`pointer-events-none block h-4 w-4 rounded-full bg-background shadow-lg ring-0 transition-transform ${w.status === 'active' ? 'translate-x-4' : 'translate-x-0'}`}
                                                        />
                                                    </button>
                                                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium uppercase tracking-wider ${statusBadgeVariant[w.status] || ''}`}>
                                                        {w.status}
                                                    </span>
                                                </div>
                                            </TableCell>
                                            <TableCell className="hidden md:table-cell">{w.steps?.length ?? 0}</TableCell>
                                            <TableCell className="hidden md:table-cell">v{w.version}</TableCell>
                                            <TableCell className="hidden lg:table-cell text-muted-foreground text-sm">
                                                {w.updated_at ? timeAgo(w.updated_at) : '—'}
                                            </TableCell>
                                            <TableCell onClick={(e) => e.stopPropagation()}>
                                                <DropdownMenu>
                                                    <DropdownMenuTrigger asChild>
                                                        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                                                            <MoreHorizontal className="h-4 w-4" />
                                                        </Button>
                                                    </DropdownMenuTrigger>
                                                    <DropdownMenuContent align="end">
                                                        <DropdownMenuItem onClick={() => navigate(`/workflows/${w.id}`)}>
                                                            Edit
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem onClick={() => handleDuplicate(w)}>
                                                            Duplicate
                                                        </DropdownMenuItem>
                                                        <DropdownMenuItem
                                                            className="text-destructive"
                                                            onClick={() => setDeleteTarget(w)}
                                                        >
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
                    )}
                </>
            )}

            {/* Delete confirm */}
            <ConfirmDialog
                open={!!deleteTarget}
                onOpenChange={() => setDeleteTarget(null)}
                title="Delete Workflow"
                description={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
                confirmLabel="Delete"
                variant="destructive"
                loading={deleting}
                onConfirm={handleDelete}
            />
        </div>
    );
};

export default WorkflowsList;
