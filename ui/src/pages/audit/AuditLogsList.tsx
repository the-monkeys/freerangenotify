import React, { useState, useCallback } from 'react';
import { auditAPI } from '../../services/api';
import type { AuditLog, AuditLogFilters, AuditAction } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { Button } from '../../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/ui/select';
import { Badge } from '../../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';
import { Pagination } from '../../components/Pagination';
import { SlidePanel } from '../../components/ui/slide-panel';
import EmptyState from '../../components/EmptyState';
import SkeletonTable from '../../components/SkeletonTable';
import { ScrollText, Search, X } from 'lucide-react';

const PAGE_SIZE = 25;

const ACTION_COLORS: Record<string, string> = {
    create: 'bg-green-100 text-green-800 border-green-300',
    update: 'bg-blue-100 text-blue-800 border-blue-300',
    delete: 'bg-red-100 text-red-800 border-red-300',
    login: 'bg-violet-100 text-violet-800 border-violet-300',
    invite: 'bg-amber-100 text-amber-800 border-amber-300',
};

const AuditLogsList: React.FC = () => {
    const [page, setPage] = useState(1);
    const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);

    // Filter form state
    const [filterAction, setFilterAction] = useState<AuditAction | ''>('');
    const [filterResource, setFilterResource] = useState('');
    const [filterAppId, setFilterAppId] = useState('');
    const [filterFrom, setFilterFrom] = useState('');
    const [filterTo, setFilterTo] = useState('');

    const activeFilters: AuditLogFilters = {
        action: filterAction || undefined,
        resource: filterResource || undefined,
        app_id: filterAppId || undefined,
        from_date: filterFrom || undefined,
        to_date: filterTo || undefined,
        offset: (page - 1) * PAGE_SIZE,
        limit: PAGE_SIZE,
    };

    const fetcher = useCallback(() => auditAPI.list(activeFilters), [
        page, filterAction, filterResource, filterAppId, filterFrom, filterTo,
    ]);
    const { data, loading } = useApiQuery<{ logs: AuditLog[]; total: number }>(
        fetcher,
        [page, filterAction, filterResource, filterAppId, filterFrom, filterTo],
    );

    const logs = data?.logs || [];
    const total = data?.total || 0;

    const handleApplyFilters = () => {
        setPage(1);
    };

    const handleClearFilters = () => {
        setFilterAction('');
        setFilterResource('');
        setFilterAppId('');
        setFilterFrom('');
        setFilterTo('');
        setPage(1);
    };

    const hasActiveFilters = filterAction || filterResource || filterAppId || filterFrom || filterTo;

    const renderValue = (val: any): string => {
        if (val === null || val === undefined) return '—';
        if (typeof val === 'object') return JSON.stringify(val, null, 2);
        return String(val);
    };

    return (
        <div className="space-y-6">
            <div className="flex items-center gap-3">
                <ScrollText className="h-6 w-6" />
                <h1 className="text-2xl font-bold text-foreground">Audit Logs</h1>
            </div>

            {/* Filters */}
            <Card>
                <CardHeader>
                    <CardTitle className="text-sm font-medium">Filters</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="grid grid-cols-1 md:grid-cols-5 gap-3 items-end">
                        <div className="space-y-1">
                            <Label className="text-xs">Action</Label>
                            <Select value={filterAction} onValueChange={v => setFilterAction(v as AuditAction)}>
                                <SelectTrigger>
                                    <SelectValue placeholder="All actions" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="create">Create</SelectItem>
                                    <SelectItem value="update">Update</SelectItem>
                                    <SelectItem value="delete">Delete</SelectItem>
                                    <SelectItem value="login">Login</SelectItem>
                                    <SelectItem value="invite">Invite</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs">Resource</Label>
                            <Select value={filterResource} onValueChange={setFilterResource}>
                                <SelectTrigger>
                                    <SelectValue placeholder="All resources" />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="application">Application</SelectItem>
                                    <SelectItem value="template">Template</SelectItem>
                                    <SelectItem value="user">User</SelectItem>
                                    <SelectItem value="notification">Notification</SelectItem>
                                    <SelectItem value="provider">Provider</SelectItem>
                                    <SelectItem value="environment">Environment</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs">App ID</Label>
                            <Input
                                value={filterAppId}
                                onChange={e => setFilterAppId(e.target.value)}
                                placeholder="Filter by app…"
                                className="h-9"
                            />
                        </div>
                        <div className="space-y-1">
                            <Label className="text-xs">From</Label>
                            <Input
                                type="date"
                                value={filterFrom}
                                onChange={e => setFilterFrom(e.target.value)}
                                className="h-9"
                            />
                        </div>
                        <div className="flex gap-2">
                            <div className="flex-1 space-y-1">
                                <Label className="text-xs">To</Label>
                                <Input
                                    type="date"
                                    value={filterTo}
                                    onChange={e => setFilterTo(e.target.value)}
                                    className="h-9"
                                />
                            </div>
                            <div className="flex items-end gap-1">
                                <Button size="sm" onClick={handleApplyFilters} className="h-9">
                                    <Search className="h-4 w-4" />
                                </Button>
                                {hasActiveFilters && (
                                    <Button variant="ghost" size="sm" onClick={handleClearFilters} className="h-9">
                                        <X className="h-4 w-4" />
                                    </Button>
                                )}
                            </div>
                        </div>
                    </div>
                </CardContent>
            </Card>

            {/* Logs Table */}
            <Card>
                <CardContent className="pt-6">
                    {loading ? (
                        <SkeletonTable columns={6} />
                    ) : logs.length === 0 ? (
                        <EmptyState
                            title="No audit logs"
                            description={hasActiveFilters ? 'No logs match your filters. Try adjusting them.' : 'Audit logs will appear here as actions are performed.'}
                        />
                    ) : (
                        <>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Time</TableHead>
                                        <TableHead>Action</TableHead>
                                        <TableHead>Resource</TableHead>
                                        <TableHead>Resource ID</TableHead>
                                        <TableHead>Actor</TableHead>
                                        <TableHead>IP</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {logs.map(log => (
                                        <TableRow
                                            key={log.audit_id}
                                            className="cursor-pointer hover:bg-muted/50"
                                            onClick={() => setSelectedLog(log)}
                                        >
                                            <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                                                {log.created_at ? new Date(log.created_at).toLocaleString() : '—'}
                                            </TableCell>
                                            <TableCell>
                                                <Badge
                                                    variant="outline"
                                                    className={`text-xs ${ACTION_COLORS[log.action] || ''}`}
                                                >
                                                    {log.action}
                                                </Badge>
                                            </TableCell>
                                            <TableCell className="text-sm">{log.resource}</TableCell>
                                            <TableCell className="text-xs font-mono text-muted-foreground max-w-[120px] truncate">
                                                {log.resource_id || '—'}
                                            </TableCell>
                                            <TableCell className="text-sm">{log.actor_id}</TableCell>
                                            <TableCell className="text-xs text-muted-foreground">
                                                {log.ip_address || '—'}
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                </TableBody>
                            </Table>
                            <div className="mt-4">
                                <Pagination
                                    currentPage={page}
                                    totalItems={total}
                                    pageSize={PAGE_SIZE}
                                    onPageChange={setPage}
                                />
                            </div>
                        </>
                    )}
                </CardContent>
            </Card>

            {/* Detail Slide Panel */}
            <SlidePanel
                open={!!selectedLog}
                onClose={() => setSelectedLog(null)}
                title="Audit Log Detail"
            >
                {selectedLog && (
                    <div className="space-y-4 p-1">
                        <div className="grid grid-cols-2 gap-3">
                            <div>
                                <p className="text-xs text-muted-foreground">Action</p>
                                <Badge
                                    variant="outline"
                                    className={`text-xs mt-0.5 ${ACTION_COLORS[selectedLog.action] || ''}`}
                                >
                                    {selectedLog.action}
                                </Badge>
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Resource</p>
                                <p className="text-sm font-medium">{selectedLog.resource}</p>
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Actor</p>
                                <p className="text-sm">{selectedLog.actor_id}</p>
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Actor Type</p>
                                <p className="text-sm">{selectedLog.actor_type}</p>
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Resource ID</p>
                                <p className="text-sm font-mono">{selectedLog.resource_id || '—'}</p>
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Timestamp</p>
                                <p className="text-sm">
                                    {selectedLog.created_at ? new Date(selectedLog.created_at).toLocaleString() : '—'}
                                </p>
                            </div>
                            {selectedLog.app_id && (
                                <div>
                                    <p className="text-xs text-muted-foreground">App ID</p>
                                    <p className="text-sm font-mono">{selectedLog.app_id}</p>
                                </div>
                            )}
                            {selectedLog.ip_address && (
                                <div>
                                    <p className="text-xs text-muted-foreground">IP Address</p>
                                    <p className="text-sm">{selectedLog.ip_address}</p>
                                </div>
                            )}
                            {selectedLog.user_agent && (
                                <div className="col-span-2">
                                    <p className="text-xs text-muted-foreground">User Agent</p>
                                    <p className="text-sm text-muted-foreground truncate">{selectedLog.user_agent}</p>
                                </div>
                            )}
                        </div>

                        {selectedLog.changes && Object.keys(selectedLog.changes).length > 0 && (
                            <div className="space-y-2">
                                <p className="text-xs text-muted-foreground font-medium">Changes</p>
                                {Object.entries(selectedLog.changes).map(([field, change]) => (
                                    <div key={field} className="space-y-1">
                                        <p className="text-sm font-medium text-foreground">{field}</p>
                                        {typeof change === 'object' && change !== null && 'old' in change && 'new' in change ? (
                                            <>
                                                <div className="bg-red-50 border-l-2 border-red-400 px-3 py-1.5 rounded-r">
                                                    <pre className="whitespace-pre-wrap text-xs text-red-800 font-mono">
                                                        − {renderValue((change as any).old)}
                                                    </pre>
                                                </div>
                                                <div className="bg-green-50 border-l-2 border-green-400 px-3 py-1.5 rounded-r">
                                                    <pre className="whitespace-pre-wrap text-xs text-green-800 font-mono">
                                                        + {renderValue((change as any).new)}
                                                    </pre>
                                                </div>
                                            </>
                                        ) : (
                                            <div className="bg-muted px-3 py-1.5 rounded border border-border">
                                                <pre className="whitespace-pre-wrap text-xs font-mono">
                                                    {renderValue(change)}
                                                </pre>
                                            </div>
                                        )}
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>
                )}
            </SlidePanel>
        </div>
    );
};

export default AuditLogsList;
