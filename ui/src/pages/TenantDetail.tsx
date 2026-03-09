import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { tenantsAPI } from '../services/api';
import type { Tenant, TenantMember } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import ConfirmDialog from '../components/ConfirmDialog';
import { Spinner } from '../components/ui/spinner';
import EmptyState from '../components/EmptyState';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';
import { ArrowLeft, Building2, UserPlus, Trash2, Shield } from 'lucide-react';

const ROLE_COLORS: Record<string, string> = {
  owner: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400',
  admin: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
  member: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
};

const TenantDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [members, setMembers] = useState<TenantMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteEmail, setInviteEmail] = useState('');
  const [inviteRole, setInviteRole] = useState<'admin' | 'member'>('member');
  const [inviting, setInviting] = useState(false);
  const [removeConfirm, setRemoveConfirm] = useState<TenantMember | null>(null);
  const [removeLoading, setRemoveLoading] = useState(false);

  useEffect(() => {
    if (id) {
      fetchTenant();
      fetchMembers();
    }
  }, [id]);

  const fetchTenant = async () => {
    if (!id) return;
    try {
      const data = await tenantsAPI.get(id);
      setTenant(data);
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to load organization'));
      navigate('/tenants');
    } finally {
      setLoading(false);
    }
  };

  const fetchMembers = async () => {
    if (!id) return;
    try {
      const data = await tenantsAPI.listMembers(id);
      setMembers(Array.isArray(data) ? data : []);
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to load members'));
    }
  };

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id || !inviteEmail.trim()) return;
    setInviting(true);
    try {
      await tenantsAPI.inviteMember(id, { email: inviteEmail.trim(), role: inviteRole });
      toast.success(`Invited ${inviteEmail.trim()} as ${inviteRole}`);
      setInviteOpen(false);
      setInviteEmail('');
      setInviteRole('member');
      fetchMembers();
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to invite member'));
    } finally {
      setInviting(false);
    }
  };

  const handleRoleChange = async (member: TenantMember, newRole: 'owner' | 'admin' | 'member') => {
    if (!id) return;
    try {
      await tenantsAPI.updateMemberRole(id, member.id, newRole);
      toast.success(`Updated ${member.user_email} to ${newRole}`);
      fetchMembers();
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to update role'));
    }
  };

  const handleRemove = async () => {
    if (!id || !removeConfirm) return;
    setRemoveLoading(true);
    try {
      await tenantsAPI.removeMember(id, removeConfirm.id);
      toast.success(`Removed ${removeConfirm.user_email}`);
      setRemoveConfirm(null);
      fetchMembers();
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to remove member'));
    } finally {
      setRemoveLoading(false);
    }
  };

  if (loading || !tenant) {
    return (
      <div className="flex justify-center items-center py-12">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <Button variant="ghost" size="sm" className="mb-4" onClick={() => navigate('/tenants')}>
        <ArrowLeft className="h-4 w-4 mr-2" />
        Back to Organizations
      </Button>

      <div className="flex items-center gap-3 mb-6">
        <div className="rounded-lg bg-muted p-3">
          <Building2 className="h-8 w-8 text-muted-foreground" />
        </div>
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-foreground">{tenant.name}</h1>
          <p className="text-sm text-muted-foreground">{tenant.slug}</p>
        </div>
      </div>

      <Tabs defaultValue="members">
        <TabsList>
          <TabsTrigger value="members">
            <Shield className="h-4 w-4 mr-2" />
            Members
          </TabsTrigger>
        </TabsList>
        <TabsContent value="members">
          <Card>
            <CardHeader>
              <div className="flex justify-between items-center">
                <CardTitle>Team Members</CardTitle>
                <Button size="sm" onClick={() => setInviteOpen(true)}>
                  <UserPlus className="h-4 w-4 mr-2" />
                  Invite
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {inviteOpen && (
                <form onSubmit={handleInvite} className="mb-6 p-4 rounded-lg border border-border bg-muted/30 space-y-4">
                  <h4 className="font-medium">Invite member</h4>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div className="md:col-span-2 space-y-2">
                      <Label htmlFor="invite-email">Email</Label>
                      <Input
                        id="invite-email"
                        type="email"
                        value={inviteEmail}
                        onChange={(e) => setInviteEmail(e.target.value)}
                        placeholder="user@example.com"
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="invite-role">Role</Label>
                      <Select value={inviteRole} onValueChange={(v) => setInviteRole(v as 'admin' | 'member')}>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="admin">Admin</SelectItem>
                          <SelectItem value="member">Member</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  <div className="flex gap-2">
                    <Button type="submit" disabled={inviting}>
                      {inviting ? 'Inviting...' : 'Send Invite'}
                    </Button>
                    <Button type="button" variant="outline" onClick={() => setInviteOpen(false)}>
                      Cancel
                    </Button>
                  </div>
                </form>
              )}

              {members.length === 0 ? (
                <EmptyState
                  title="No members yet"
                  description="Invite team members to give them access to this organization."
                  action={{ label: 'Invite Member', onClick: () => setInviteOpen(true) }}
                />
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Email</TableHead>
                      <TableHead>Role</TableHead>
                      <TableHead className="w-[120px]">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(members ?? []).map((member) => (
                      <TableRow key={member.id}>
                        <TableCell>{member.user_email}</TableCell>
                        <TableCell>
                          {member.role === 'owner' ? (
                            <Badge className={ROLE_COLORS.owner}>Owner</Badge>
                          ) : (
                            <Select
                              value={member.role}
                              onValueChange={(v) => handleRoleChange(member, v as 'owner' | 'admin' | 'member')}
                            >
                              <SelectTrigger className="w-[120px]">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="owner">Owner</SelectItem>
                                <SelectItem value="admin">Admin</SelectItem>
                                <SelectItem value="member">Member</SelectItem>
                              </SelectContent>
                            </Select>
                          )}
                        </TableCell>
                        <TableCell>
                          {member.role !== 'owner' && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive hover:text-destructive"
                              onClick={() => setRemoveConfirm(member)}
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
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <ConfirmDialog
        open={!!removeConfirm}
        onOpenChange={(open) => !open && setRemoveConfirm(null)}
        title="Remove member"
        description={
          removeConfirm
            ? `Are you sure you want to remove ${removeConfirm.user_email} from this organization?`
            : ''
        }
        confirmLabel="Remove"
        variant="destructive"
        loading={removeLoading}
        onConfirm={handleRemove}
      />
    </div>
  );
};

export default TenantDetail;
