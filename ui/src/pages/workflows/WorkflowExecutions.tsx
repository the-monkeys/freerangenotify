import React, { useState } from 'react';
import { workflowsAPI, applicationsAPI } from '../../services/api';
import type { WorkflowExecution, ExecutionStatus, Application } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import ResourcePicker from '../../components/ResourcePicker';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import ExecutionTimeline from '../../components/workflows/ExecutionTimeline';
import { Pagination } from '../../components/Pagination';
import { Badge } from '../../components/ui/badge';
import { Button } from '../../components/ui/button';
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
import { ChevronDown, ChevronRight, Activity, StopCircle, Loader2 } from 'lucide-react';
import { timeAgo } from '../../lib/utils';
import { toast } from 'sonner';

const statusBadge: Record<ExecutionStatus, { variant: 'default' | 'secondary' | 'destructive' | 'outline'; label: string }> = {
    running: { variant: 'default', label: 'Running' },
    paused: { variant: 'secondary', label: 'Paused' },
    completed: { variant: 'outline', label: 'Completed' },
    failed: { variant: 'destructive', label: 'Failed' },
    cancelled: { variant: 'secondary', label: 'Cancelled' },
};

const PAGE_SIZE = 15;

const WorkflowExecutions: React.FC = () => {
    const [apiKey, setApiKey] = useState<string | null>(
        localStorage.getItem('last_api_key')
    );
    const [selectedAppId, setSelectedAppId] = useState<string | null>(
        localStorage.getItem('last_app_id')
    );
    const [statusFilter, setStatusFilter] = useState<ExecutionStatus | 'all'>('all');
    const [page, setPage] = useState(1);
    const [expandedId, setExpandedId] = useState<string | null>(null);
    const [cancellingId, setCancellingId] = useState<string | null>(null);

    const offset = (page - 1) * PAGE_SIZE;

    const { data, loading, refetch } = useApiQuery(
        () => workflowsAPI.listExecutions(apiKey!, PAGE_SIZE, offset),
        [apiKey, offset],
        { enabled: !!apiKey }
    );

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const apps = await applicationsAPI.list();
            const app = apps.find((a: Application) => a.app_id === appId);
            if (app) {
                setSelectedAppId(app.app_id);
                setApiKey(app.api_key);
                localStorage.setItem('last_app_id', app.app_id);
                localStorage.setItem('last_api_key', app.api_key);
            }
        } catch {
            toast.error('Failed to load application');
        }
    };

    const handleCancel = async (execId: string) => {
        if (!apiKey) return;
        setCancellingId(execId);
        try {
            await workflowsAPI.cancelExecution(apiKey, execId);
            toast.success('Execution cancelled');
            refetch();
        } catch {
            toast.error('Failed to cancel execution');
        } finally {
            setCancellingId(null);
        }
    };

    const executions: WorkflowExecution[] = data?.executions || [];
    const total: number = data?.total || 0;

    const filtered = statusFilter === 'all'
        ? executions
        : executions.filter(e => e.status === statusFilter);

    if (!apiKey) {
        return (
            <div className="p-6 max-w-6xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">Workflow Executions</h1>
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                        hint="Select an app to view executions"
                    />
                </div>
            </div>
        );
    }

    return (
        <div className="p-6 max-w-6xl mx-auto space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <Activity className="h-5 w-5 text-muted-foreground" />
                    <h1 className="text-2xl font-semibold text-foreground">Workflow Executions</h1>
                </div>
                <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v as ExecutionStatus | 'all')}>
                    <SelectTrigger className="w-[160px]">
                        <SelectValue placeholder="All statuses" />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">All Statuses</SelectItem>
                        <SelectItem value="running">Running</SelectItem>
                        <SelectItem value="paused">Paused</SelectItem>
                        <SelectItem value="completed">Completed</SelectItem>
                        <SelectItem value="failed">Failed</SelectItem>
                        <SelectItem value="cancelled">Cancelled</SelectItem>
                    </SelectContent>
                </Select>
            </div>

            {/* Table */}
            {loading ? (
                <SkeletonTable rows={6} columns={5} />
            ) : filtered.length === 0 ? (
                <EmptyState
                    title="No executions found"
                    description={statusFilter !== 'all' ? 'Try a different status filter' : 'Trigger a workflow to see executions here'}
                />
            ) : (
                <>
                    <div className="border border-border rounded-lg overflow-hidden">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead className="w-8" />
                                    <TableHead>Execution ID</TableHead>
                                    <TableHead>Workflow</TableHead>
                                    <TableHead>User</TableHead>
                                    <TableHead>Status</TableHead>
                                    <TableHead>Started</TableHead>
                                    <TableHead className="text-right">Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {filtered.map((exec) => {
                                    const isExpanded = expandedId === exec.id;
                                    const badge = statusBadge[exec.status] || statusBadge.running;

                                    return (
                                        <React.Fragment key={exec.id}>
                                            <TableRow
                                                className="cursor-pointer hover:bg-muted/50"
                                                onClick={() => setExpandedId(isExpanded ? null : exec.id)}
                                            >
                                                <TableCell>
                                                    {isExpanded
                                                        ? <ChevronDown className="h-4 w-4 text-muted-foreground" />
                                                        : <ChevronRight className="h-4 w-4 text-muted-foreground" />
                                                    }
                                                </TableCell>
                                                <TableCell className="font-mono text-xs">
                                                    {exec.id.slice(0, 12)}…
                                                </TableCell>
                                                <TableCell className="font-mono text-xs text-muted-foreground">
                                                    {exec.workflow_id.slice(0, 12)}…
                                                </TableCell>
                                                <TableCell className="font-mono text-xs text-muted-foreground">
                                                    {exec.user_id.slice(0, 12)}…
                                                </TableCell>
                                                <TableCell>
                                                    <Badge variant={badge.variant}>{badge.label}</Badge>
                                                </TableCell>
                                                <TableCell className="text-xs text-muted-foreground">
                                                    {timeAgo(exec.started_at)}
                                                </TableCell>
                                                <TableCell className="text-right">
                                                    {exec.status === 'running' && (
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            onClick={(e) => {
                                                                e.stopPropagation();
                                                                handleCancel(exec.id);
                                                            }}
                                                            disabled={cancellingId === exec.id}
                                                        >
                                                            {cancellingId === exec.id
                                                                ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                                                                : <StopCircle className="h-3.5 w-3.5" />
                                                            }
                                                        </Button>
                                                    )}
                                                </TableCell>
                                            </TableRow>

                                            {/* Expanded detail row */}
                                            {isExpanded && (
                                                <TableRow>
                                                    <TableCell colSpan={7} className="bg-muted/30 px-8 py-4">
                                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                                            <div>
                                                                <h4 className="text-sm font-medium text-foreground mb-3">
                                                                    Step Timeline
                                                                </h4>
                                                                <ExecutionTimeline stepResults={exec.step_results} />
                                                            </div>
                                                            <div>
                                                                <h4 className="text-sm font-medium text-foreground mb-2">
                                                                    Payload
                                                                </h4>
                                                                <pre className="text-xs bg-muted p-3 rounded-md overflow-auto max-h-48 font-mono">
                                                                    {JSON.stringify(exec.payload, null, 2)}
                                                                </pre>
                                                                {exec.transaction_id && (
                                                                    <p className="mt-2 text-xs text-muted-foreground">
                                                                        Transaction: <span className="font-mono">{exec.transaction_id}</span>
                                                                    </p>
                                                                )}
                                                            </div>
                                                        </div>
                                                    </TableCell>
                                                </TableRow>
                                            )}
                                        </React.Fragment>
                                    );
                                })}
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
        </div>
    );
};

export default WorkflowExecutions;
