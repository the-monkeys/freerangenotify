import React, { useState, useMemo, useRef } from 'react';
import { useApiQuery } from '../hooks/use-api-query';
import { usersAPI } from '../services/api';
import type { User, CreateUserRequest } from '../types';
import { extractErrorMessage } from '../lib/utils';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Checkbox } from './ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { Pagination } from './Pagination';
import ConfirmDeleteDialog from './ConfirmDeleteDialog';
import { toast } from 'sonner';
import { Edit, Trash2, Upload, Download } from 'lucide-react';

const getBrowserTimezone = () => Intl.DateTimeFormat().resolvedOptions().timeZone;

const TIMEZONES_BASE = [
    'UTC', 'America/New_York', 'America/Chicago', 'America/Denver',
    'America/Los_Angeles', 'America/Toronto', 'Europe/London',
    'Europe/Paris', 'Europe/Berlin', 'Asia/Tokyo', 'Asia/Shanghai',
    'Asia/Kolkata', 'Asia/Dubai', 'Australia/Sydney', 'Pacific/Auckland',
];

const getTimezones = (): string[] => {
    const browser = getBrowserTimezone();
    if (TIMEZONES_BASE.includes(browser)) return TIMEZONES_BASE;
    return [browser, ...TIMEZONES_BASE];
};

const LANGUAGES = [
    { code: 'en', label: 'English' },
    { code: 'es', label: 'Spanish' },
    { code: 'fr', label: 'French' },
    { code: 'de', label: 'German' },
    { code: 'pt', label: 'Portuguese' },
    { code: 'zh', label: 'Chinese' },
    { code: 'ja', label: 'Japanese' },
    { code: 'ko', label: 'Korean' },
    { code: 'ar', label: 'Arabic' },
    { code: 'hi', label: 'Hindi' },
];

interface AppUsersProps {
    apiKey: string;
}

const AppUsers: React.FC<AppUsersProps> = ({ apiKey }) => {
    const [showAddForm, setShowAddForm] = useState(false);
    const [editingUser, setEditingUser] = useState<string | null>(null);
    const [page, setPage] = useState(1);
    const [pageSize] = useState(20);
    const [deleteTarget, setDeleteTarget] = useState<User | null>(null);
    const [deleteLoading, setDeleteLoading] = useState(false);
    const fileInputRef = useRef<HTMLInputElement>(null);
    const [uploadingCSV, setUploadingCSV] = useState(false);

    const {
        data: usersData,
        loading,
        refetch: fetchUsers
    } = useApiQuery(
        () => usersAPI.list(apiKey, page, pageSize),
        [apiKey, page, pageSize],
        {
            cacheKey: `users-${apiKey}-${page}`,
            staleTime: 60000 // 1 minute
        }
    );

    const users = useMemo(() => usersData?.users || [], [usersData]);
    const totalCount = useMemo(() => usersData?.total_count || 0, [usersData]);
    const [formData, setFormData] = useState<CreateUserRequest>({
        email: '',
        full_name: '',
        phone: '',
        external_id: '',
        timezone: getBrowserTimezone(),
        language: 'en',
        preferences: {
            email_enabled: true,
            push_enabled: true,
            sms_enabled: true,
            dnd: false,
            daily_limit: 0,
            quiet_hours: {
                enabled: false,
                start: '',
                end: ''
            }
        }
    });



    const handleUpdateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            if (editingUser) {
                // Update
                const updatePayload: any = {
                    external_id: formData.external_id,
                    full_name: formData.full_name,
                    email: formData.email,
                    phone: formData.phone,
                    timezone: formData.timezone,
                    language: formData.language,
                    preferences: formData.preferences,
                };
                await usersAPI.update(apiKey, editingUser, updatePayload);
                setEditingUser(null);
            } else {
                // Create
                await usersAPI.create(apiKey, formData);
            }
            setShowAddForm(false);
            setFormData({
                email: '',
                full_name: '',
                phone: '',
                external_id: '',
                timezone: getBrowserTimezone(),
                language: 'en',
                preferences: {
                    email_enabled: true,
                    push_enabled: true,
                    sms_enabled: true,
                    dnd: false,
                    daily_limit: 0,
                    quiet_hours: {
                        enabled: false,
                        start: '',
                        end: ''
                    }
                }
            });
            fetchUsers();
            toast.success(editingUser ? 'User updated successfully!' : 'User created successfully!');
        } catch (error) {
            console.error('Failed to save user:', error);
            toast.error(extractErrorMessage(error, 'Failed to save user'));
        }
    };

    const handleEditClick = (user: User) => {
        setEditingUser(user.user_id);
        const qh = user.preferences?.quiet_hours;
        setFormData({
            user_id: user.user_id,
            external_id: user.external_id || '',
            full_name: user.full_name || '',
            email: user.email,
            phone: user.phone || '',
            timezone: user.timezone || getBrowserTimezone(),
            language: user.language || 'en',
            preferences: {
                email_enabled: user.preferences?.email_enabled ?? true,
                push_enabled: user.preferences?.push_enabled ?? true,
                sms_enabled: user.preferences?.sms_enabled ?? true,
                dnd: user.preferences?.dnd ?? false,
                daily_limit: user.preferences?.daily_limit ?? 0,
                quiet_hours: {
                    enabled: qh?.enabled ?? !!(qh?.start || qh?.end),
                    start: qh?.start || '',
                    end: qh?.end || '',
                }
            }
        });
        setShowAddForm(true);
    };

    const handleDeleteUser = (user: User) => {
        setDeleteTarget(user);
    };

    const handleConfirmDeleteUser = async () => {
        if (!deleteTarget) return;
        setDeleteLoading(true);
        try {
            await usersAPI.delete(apiKey, deleteTarget.user_id);
            fetchUsers();
            setDeleteTarget(null);
            toast.success('User deleted successfully');
        } catch (error) {
            console.error('Failed to delete user:', error);
            toast.error(extractErrorMessage(error, 'Failed to delete user'));
        } finally {
            setDeleteLoading(false);
        }
    };

    const handleCSVUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        if (!file) return;

        setUploadingCSV(true);
        const reader = new FileReader();
        reader.onload = async (e) => {
            try {
                const text = e.target?.result as string;
                if (!text) throw new Error("File empty");

                const lines = text.split('\n').filter(l => l.trim().length > 0);
                if (lines.length < 2) throw new Error("CSV must contain a header row and at least one data row.");

                const headers = lines[0].split(',').map(h => h.trim().toLowerCase());
                const emailIdx = headers.indexOf('email');
                const nameIdx = headers.indexOf('full_name') !== -1 ? headers.indexOf('full_name') : headers.indexOf('name');
                const phoneIdx = headers.indexOf('phone');
                const extIdIdx = headers.indexOf('external_id');

                if (emailIdx === -1) {
                    throw new Error("CSV header must contain 'email'");
                }

                const usersToCreate: CreateUserRequest[] = [];
                for (let i = 1; i < lines.length; i++) {
                    const columns = lines[i].split(',').map(c => c.trim());
                    if (columns.length < headers.length) continue;

                    if (!columns[emailIdx]) continue; // Skip empty emails

                    usersToCreate.push({
                        email: columns[emailIdx],
                        full_name: nameIdx !== -1 ? columns[nameIdx] : undefined,
                        phone: phoneIdx !== -1 ? columns[phoneIdx] : undefined,
                        external_id: extIdIdx !== -1 ? columns[extIdIdx] : undefined,
                        timezone: getBrowserTimezone(),
                        language: 'en',
                    });
                }

                if (usersToCreate.length === 0) {
                    throw new Error("No valid valid user data rows found.");
                }

                const res = await usersAPI.bulkCreate(apiKey, {
                    upsert: false,
                    skip_existing: true,
                    users: usersToCreate
                });

                toast.success(`Import successful: ${res.created} created, ${res.skipped} skipped.`);
                fetchUsers();
            } catch (err) {
                console.error("CSV import error", err);
                toast.error(extractErrorMessage(err, "Failed to parse or upload CSV"));
            } finally {
                setUploadingCSV(false);
                if (fileInputRef.current) fileInputRef.current.value = '';
            }
        };
        reader.onerror = () => {
            toast.error("Failed to read file.");
            setUploadingCSV(false);
            if (fileInputRef.current) fileInputRef.current.value = '';
        };
        reader.readAsText(file);
    };

    const handleDownloadCSVTemplate = () => {
        const csvContent = "email,full_name,phone,external_id\nuser1@example.com,John Doe,+15550100,ext_123\nuser2@example.com,Jane Smith,,ext_124\n";
        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.setAttribute('download', 'users_import_template.csv');
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
    };

    if (loading) return <div className="text-center py-4">Loading users...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3">
                    <CardTitle>Application Users</CardTitle>
                    <div className="flex flex-wrap items-center gap-2">
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={handleDownloadCSVTemplate}
                            title="Download CSV template"
                        >
                            <Download className="h-4 w-4 mr-1" />
                            CSV Template
                        </Button>
                        <input
                            type="file"
                            accept=".csv"
                            className="hidden"
                            ref={fileInputRef}
                            onChange={handleCSVUpload}
                        />
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => fileInputRef.current?.click()}
                            disabled={uploadingCSV}
                        >
                            <Upload className="h-4 w-4 mr-1" />
                            {uploadingCSV ? 'Importing...' : 'Import CSV'}
                        </Button>
                        <Button
                            size="sm"
                            onClick={() => {
                                setShowAddForm(!showAddForm);
                                if (!showAddForm) {
                                    setEditingUser(null);
                                    setFormData({
                                        email: '',
                                        full_name: '',
                                        phone: '',
                                        external_id: '',
                                        timezone: getBrowserTimezone(),
                                        language: 'en',
                                        preferences: {
                                            email_enabled: true,
                                            push_enabled: true,
                                            sms_enabled: true,
                                            dnd: false,
                                            daily_limit: 0,
                                            quiet_hours: { enabled: false, start: '', end: '' }
                                        }
                                    });
                                }
                            }}
                        >
                            {showAddForm ? 'Cancel' : 'Add User'}
                        </Button>
                    </div>
                </div>
            </CardHeader>
            <CardContent>
                {showAddForm && (
                    <form onSubmit={handleUpdateUser} className="mb-8 bg-muted p-6 rounded border border-border space-y-4">
                        <h4 className="text-lg font-semibold mb-4">{editingUser ? 'Edit User' : 'Add New User'}</h4>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="user_id">Custom User ID (Internal)</Label>
                                <Input
                                    id="user_id"
                                    type="text"
                                    value={formData.user_id || ''}
                                    onChange={(e) => setFormData({ ...formData, user_id: e.target.value })}
                                    placeholder="e.g. EMP_001 (Optional)"
                                    disabled={!!editingUser}
                                    className={editingUser ? "bg-muted cursor-not-allowed" : ""}
                                />
                                {!editingUser && (
                                    <p className="text-xs text-muted-foreground">
                                        Leave empty to auto-generate a UUID.
                                    </p>
                                )}
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="external_id">External ID</Label>
                                <Input
                                    id="external_id"
                                    type="text"
                                    value={formData.external_id || ''}
                                    onChange={(e) => setFormData({ ...formData, external_id: e.target.value })}
                                    placeholder="e.g. platform_user_123 (Optional)"
                                />
                                <p className="text-xs text-muted-foreground">
                                    Your platform's user identifier. Used for SSE connections and cross-system lookups.
                                </p>
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="full_name">Full Name</Label>
                                <Input
                                    id="full_name"
                                    type="text"
                                    value={formData.full_name || ''}
                                    onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                                    placeholder="John Doe"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="email">Email</Label>
                                <Input
                                    id="email"
                                    type="email"
                                    value={formData.email || ''}
                                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                                    placeholder="user@example.com"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="phone">Phone Number</Label>
                                <Input
                                    id="phone"
                                    type="tel"
                                    value={formData.phone || ''}
                                    onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                                    placeholder="+1 555 0100"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="language">Language</Label>
                                <Select
                                    value={formData.language || 'en'}
                                    onValueChange={(v) => setFormData({ ...formData, language: v })}
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {LANGUAGES.map(l => (
                                            <SelectItem key={l.code} value={l.code}>{l.label} ({l.code})</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="timezone">Timezone</Label>
                                <Select
                                    value={formData.timezone || getBrowserTimezone()}
                                    onValueChange={(v) => setFormData({ ...formData, timezone: v })}
                                >
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        {getTimezones().map(tz => (
                                            <SelectItem key={tz} value={tz}>{tz}</SelectItem>
                                        ))}
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className="col-span-full space-y-3">
                                <Label className="block mb-2">Preferences</Label>
                                <div className="flex gap-6 flex-wrap">
                                    <div className="flex items-center space-x-2">
                                        <Checkbox
                                            id="email_enabled"
                                            checked={formData.preferences?.email_enabled ?? true}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: { ...formData.preferences, email_enabled: !!checked }
                                            })}
                                        />
                                        <Label htmlFor="email_enabled" className="cursor-pointer">Email Enabled</Label>
                                    </div>
                                    <div className="flex items-center space-x-2">
                                        <Checkbox
                                            id="push_enabled"
                                            checked={formData.preferences?.push_enabled ?? true}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: { ...formData.preferences, push_enabled: !!checked }
                                            })}
                                        />
                                        <Label htmlFor="push_enabled" className="cursor-pointer">Push Enabled</Label>
                                    </div>
                                    <div className="flex items-center space-x-2">
                                        <Checkbox
                                            id="sms_enabled"
                                            checked={formData.preferences?.sms_enabled ?? true}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: { ...formData.preferences, sms_enabled: !!checked }
                                            })}
                                        />
                                        <Label htmlFor="sms_enabled" className="cursor-pointer">SMS Enabled</Label>
                                    </div>
                                    <div className="flex items-center space-x-2">
                                        <Checkbox
                                            id="dnd"
                                            checked={formData.preferences?.dnd ?? false}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: { ...formData.preferences, dnd: !!checked }
                                            })}
                                        />
                                        <Label htmlFor="dnd" className="cursor-pointer">Do Not Disturb</Label>
                                    </div>
                                </div>

                                <div className="mt-4 border-t border-border pt-4">
                                    <div className="flex items-center space-x-2 mb-2">
                                        <Checkbox
                                            id="quiet_hours_enabled"
                                            checked={formData.preferences?.quiet_hours?.enabled ?? false}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: {
                                                    ...formData.preferences,
                                                    quiet_hours: {
                                                        enabled: !!checked,
                                                        start: formData.preferences?.quiet_hours?.start ?? '',
                                                        end: formData.preferences?.quiet_hours?.end ?? '',
                                                    }
                                                }
                                            })}
                                        />
                                        <Label htmlFor="quiet_hours_enabled" className="font-semibold cursor-pointer">Quiet Hours</Label>
                                    </div>

                                    {formData.preferences?.quiet_hours?.enabled && (
                                        <div className="flex gap-4 ml-6">
                                            <div className="space-y-1">
                                                <Label htmlFor="quiet_start" className="text-xs">Start Time</Label>
                                                <Input
                                                    id="quiet_start"
                                                    type="time"
                                                    className="w-auto"
                                                    value={formData.preferences?.quiet_hours?.start || ''}
                                                    onChange={(e) => setFormData({
                                                        ...formData,
                                                        preferences: {
                                                            ...formData.preferences,
                                                            quiet_hours: {
                                                                enabled: formData.preferences?.quiet_hours?.enabled ?? false,
                                                                start: e.target.value,
                                                                end: formData.preferences?.quiet_hours?.end ?? '',
                                                            }
                                                        }
                                                    })}
                                                />
                                            </div>
                                            <div className="space-y-1">
                                                <Label htmlFor="quiet_end" className="text-xs">End Time</Label>
                                                <Input
                                                    id="quiet_end"
                                                    type="time"
                                                    className="w-auto"
                                                    value={formData.preferences?.quiet_hours?.end || ''}
                                                    onChange={(e) => setFormData({
                                                        ...formData,
                                                        preferences: {
                                                            ...formData.preferences,
                                                            quiet_hours: {
                                                                enabled: formData.preferences?.quiet_hours?.enabled ?? false,
                                                                start: formData.preferences?.quiet_hours?.start ?? '',
                                                                end: e.target.value,
                                                            }
                                                        }
                                                    })}
                                                />
                                            </div>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className="flex justify-end mt-6">
                            <Button type="submit">{editingUser ? 'Update User' : 'Create User'}</Button>
                        </div>
                    </form>
                )}

                {users.length === 0 ? (
                    <p className="text-muted-foreground text-center py-8 text-sm">No users found for this application.</p>
                ) : (
                    <div className="overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Email</TableHead>
                                    <TableHead>Name</TableHead>
                                    <TableHead>Phone</TableHead>
                                    <TableHead>Language</TableHead>
                                    <TableHead>Quiet Hours</TableHead>
                                    <TableHead>Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {users.map((user) => (
                                    <TableRow key={user.user_id}>
                                        <TableCell className="font-medium">{user.email || '-'}</TableCell>
                                        <TableCell>{user.full_name || '-'}</TableCell>
                                        <TableCell>{user.phone || '-'}</TableCell>
                                        <TableCell>{user.language || 'en'}</TableCell>
                                        <TableCell>
                                            {user.preferences?.quiet_hours?.enabled ? (
                                                <span className="text-xs text-amber-600 font-medium">
                                                    {user.preferences.quiet_hours.start || '?'} – {user.preferences.quiet_hours.end || '?'}
                                                </span>
                                            ) : (
                                                <span className="text-xs text-muted-foreground">Off</span>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            <button
                                                onClick={() => handleEditClick(user)}
                                                className="text-foreground hover:underline font-semibold mr-4"
                                            >
                                                <Edit className="h-4 w-4 inline" />
                                            </button>
                                            <button
                                                onClick={() => handleDeleteUser(user)}
                                                className="text-red-600 hover:underline font-semibold"
                                            >
                                                <Trash2 className="h-4 w-4 inline" />
                                            </button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
                <Pagination
                    currentPage={page}
                    totalItems={totalCount}
                    pageSize={pageSize}
                    onPageChange={setPage}
                />

                <ConfirmDeleteDialog
                    open={!!deleteTarget}
                    onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}
                    title="Delete User"
                    description={deleteTarget ? `Delete user ${deleteTarget.email || deleteTarget.user_id}?` : 'Delete this user?'}
                    confirmLabel="Delete"
                    confirmVariant="destructive"
                    loading={deleteLoading}
                    onConfirm={handleConfirmDeleteUser}
                />
            </CardContent>
        </Card>
    );
};

export default AppUsers;
