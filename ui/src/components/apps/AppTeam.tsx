import React, { useState, useCallback } from 'react';
import { teamAPI } from '../../services/api';
import type { AppMembership, TeamRole } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { SlidePanel } from '../ui/slide-panel';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import SkeletonTable from '../SkeletonTable';
import { toast } from 'sonner';
import { Loader2, Shield, UserPlus, Trash2 } from 'lucide-react';

interface AppTeamProps {
    appId: string;
}

const ROLE_COLORS: Record<TeamRole, string> = {
    owner: 'bg-amber-100 text-amber-800 border-amber-300',
    admin: 'bg-blue-100 text-blue-800 border-blue-300',
    editor: 'bg-green-100 text-green-800 border-green-300',
    viewer: 'bg-gray-100 text-gray-800 border-gray-300',
};

const ROLE_OPTIONS: TeamRole[] = ['owner', 'admin', 'editor', 'viewer'];

const AppTeam: React.FC<AppTeamProps> = ({ appId }) => {
    const [inviteOpen, setInviteOpen] = useState(false);
    const [inviteEmail, setInviteEmail] = useState('');
    const [inviteRole, setInviteRole] = useState<Exclude<TeamRole, 'owner'>>('viewer');
    const [inviting, setInviting] = useState(false);
    const [removeConfirm, setRemoveConfirm] = useState<AppMembership | null>(null);
    const [removeLoading, setRemoveLoading] = useState(false);

    const fetcher = useCallback(() => teamAPI.listMembers(appId), [appId]);
    const { data: members, loading, error, refetch } = useApiQuery<AppMembership[]>(fetcher, [appId]);

    const handleInvite = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!inviteEmail.trim()) return;
        setInviting(true);
        try {
            await teamAPI.inviteMember(appId, { email: inviteEmail.trim(), role: inviteRole });
            toast.success(`Invited ${inviteEmail.trim()} as ${inviteRole}`);
            setInviteOpen(false);
            setInviteEmail('');
            setInviteRole('viewer' as Exclude<TeamRole, 'owner'>);
            refetch();
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to invite member'));
        } finally {
            setInviting(false);
        }
    };

    const handleRoleChange = async (membership: AppMembership, newRole: TeamRole) => {
        try {
            await teamAPI.updateRole(appId, membership.membership_id, { role: newRole });
            toast.success(`Updated ${membership.user_email} to ${newRole}`);
            refetch();
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to update role'));
        }
    };

    const handleRemove = async () => {
        if (!removeConfirm) return;
        setRemoveLoading(true);
        try {
            await teamAPI.removeMember(appId, removeConfirm.membership_id);
            toast.success(`Removed ${removeConfirm.user_email}`);
            setRemoveConfirm(null);
            refetch();
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to remove member'));
        } finally {
            setRemoveLoading(false);
        }
    };

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle className="flex items-center gap-2">
                        <Shield className="h-5 w-5" />
                        Team Members
                    </CardTitle>
                    <Button size="sm" onClick={() => setInviteOpen(true)}>
                        <UserPlus className="h-4 w-4 mr-2" />
                        Invite
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {loading ? (
                    <SkeletonTable columns={4} />
                ) : error ? (
                    <div className="text-center py-8">
                        <p className="text-sm text-destructive">{error}</p>
                        <Button variant="outline" size="sm" className="mt-2" onClick={refetch}>Retry</Button>
                    </div>
                ) : !members || members.length === 0 ? (
                    <EmptyState
                        title="No team members"
                        description="Invite team members to collaborate on this application."
                        action={{ label: 'Invite Member', onClick: () => setInviteOpen(true) }}
                    />
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Email</TableHead>
                                <TableHead>Role</TableHead>
                                <TableHead>Joined</TableHead>
                                <TableHead className="w-[80px]">Actions</TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {members.map(m => (
                                <TableRow key={m.membership_id}>
                                    <TableCell className="font-medium">{m.user_email}</TableCell>
                                    <TableCell>
                                        {m.role === 'owner' ? (
                                            <Badge variant="outline" className={ROLE_COLORS[m.role]}>
                                                {m.role}
                                            </Badge>
                                        ) : (
                                            <Select
                                                value={m.role}
                                                onValueChange={(v) => handleRoleChange(m, v as TeamRole)}
                                            >
                                                <SelectTrigger className="h-8 w-[120px]" aria-label="Change role">
                                                    <SelectValue />
                                                </SelectTrigger>
                                                <SelectContent>
                                                    {ROLE_OPTIONS.filter(r => r !== 'owner').map(r => (
                                                        <SelectItem key={r} value={r}>{r}</SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                        )}
                                    </TableCell>
                                    <TableCell className="text-sm text-muted-foreground">
                                        {m.created_at ? new Date(m.created_at).toLocaleDateString() : '—'}
                                    </TableCell>
                                    <TableCell>
                                        {m.role !== 'owner' && (
                                            <Button
                                                variant="ghost"
                                                size="sm"
                                                className="text-destructive hover:text-destructive"
                                                onClick={() => setRemoveConfirm(m)}
                                                aria-label="Remove team member"
                                            >
                                                <Trash2 className="h-4 w-4" />
                                            </Button>
                                        )}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}

                {/* Role reference */}
                <div className="mt-6 p-4 bg-muted rounded border border-border">
                    <p className="text-xs font-medium text-foreground mb-2">Role Permissions</p>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs text-muted-foreground">
                        <div><Badge variant="outline" className={ROLE_COLORS.owner}>owner</Badge> Full access</div>
                        <div><Badge variant="outline" className={ROLE_COLORS.admin}>admin</Badge> Manage members</div>
                        <div><Badge variant="outline" className={ROLE_COLORS.editor}>editor</Badge> Edit resources</div>
                        <div><Badge variant="outline" className={ROLE_COLORS.viewer}>viewer</Badge> Read-only</div>
                    </div>
                </div>
            </CardContent>

            {/* Invite Panel */}
            <SlidePanel
                open={inviteOpen}
                onClose={() => setInviteOpen(false)}
                title="Invite Team Member"
            >
                <form onSubmit={handleInvite} className="space-y-4 p-1">
                    <div className="space-y-1.5">
                        <Label className="text-xs">Email Address</Label>
                        <Input
                            type="email"
                            value={inviteEmail}
                            onChange={e => setInviteEmail(e.target.value)}
                            placeholder="member@example.com"
                            required
                        />
                    </div>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Role</Label>
                        <Select value={inviteRole} onValueChange={v => setInviteRole(v as Exclude<TeamRole, 'owner'>)}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {ROLE_OPTIONS.filter(r => r !== 'owner').map(r => (
                                    <SelectItem key={r} value={r}>{r}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <Button type="submit" className="w-full" disabled={inviting}>
                        {inviting ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Send Invitation
                    </Button>
                </form>
            </SlidePanel>

            {/* Remove Confirmation */}
            <ConfirmDialog
                open={!!removeConfirm}
                onOpenChange={(open) => { if (!open) setRemoveConfirm(null); }}
                title="Remove Team Member"
                description={removeConfirm ? `Remove ${removeConfirm.user_email} from this application? They will lose all access.` : ''}
                confirmLabel="Remove"
                variant="destructive"
                loading={removeLoading}
                onConfirm={handleRemove}
            />
        </Card>
    );
};

export default AppTeam;
