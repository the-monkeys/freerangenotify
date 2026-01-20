import React, { useEffect, useState } from 'react';
import { usersAPI } from '../services/api';
import type { User, CreateUserRequest } from '../types';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Checkbox } from './ui/checkbox';
import { toast } from 'sonner';

interface AppUsersProps {
    apiKey: string;
}

const AppUsers: React.FC<AppUsersProps> = ({ apiKey }) => {
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [editingUser, setEditingUser] = useState<string | null>(null);
    const [formData, setFormData] = useState<CreateUserRequest>({
        email: '',
        timezone: 'UTC',
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

    useEffect(() => {
        fetchUsers();
    }, [apiKey]);

    const fetchUsers = async () => {
        setLoading(true);
        try {
            const data = await usersAPI.list(apiKey);
            setUsers(data);
        } catch (error) {
            console.error('Failed to fetch users:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleUpdateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            if (editingUser) {
                // Update
                const updatePayload: any = {
                    email: formData.email,
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
                timezone: 'UTC',
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
            toast.error('Failed to save user');
        }
    };

    const handleEditClick = (user: User) => {
        setEditingUser(user.user_id);
        setFormData({
            user_id: user.user_id,
            email: user.email,
            timezone: user.timezone || 'UTC',
            language: user.language || 'en',
            preferences: {
                email_enabled: user.preferences?.email_enabled ?? true,
                push_enabled: user.preferences?.push_enabled ?? true,
                sms_enabled: user.preferences?.sms_enabled ?? true,
                dnd: user.preferences?.dnd ?? false,
                daily_limit: user.preferences?.daily_limit ?? 0,
                quiet_hours: user.preferences?.quiet_hours || { enabled: false, start: '', end: '' }
            }
        });
        setShowAddForm(true);
    };

    const handleDeleteUser = async (userId: string) => {
        if (!window.confirm('Delete this user?')) return;
        try {
            await usersAPI.delete(apiKey, userId);
            fetchUsers();
        } catch (error) {
            console.error('Failed to delete user:', error);
        }
    };

    if (loading) return <div className="text-center py-4">Loading users...</div>;

    return (
        <Card>
            <CardHeader>
                <div className="flex justify-between items-center">
                    <CardTitle>Application Users</CardTitle>
                    <Button
                        onClick={() => {
                            setShowAddForm(!showAddForm);
                            if (!showAddForm) {
                                setEditingUser(null);
                                setFormData({
                                    email: '',
                                    timezone: 'UTC',
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
                            }
                        }}
                    >
                        {showAddForm ? 'Cancel' : 'Add User'}
                    </Button>
                </div>
            </CardHeader>
            <CardContent>
                {showAddForm && (
                    <form onSubmit={handleUpdateUser} className="mb-8 bg-gray-50 p-6 rounded border border-gray-200 space-y-4">
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
                                    className={editingUser ? "bg-gray-100 cursor-not-allowed" : ""}
                                />
                                {!editingUser && (
                                    <p className="text-xs text-gray-500">
                                        Leave empty to auto-generate a UUID.
                                    </p>
                                )}
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
                                <Label htmlFor="language">Language</Label>
                                <Input
                                    id="language"
                                    type="text"
                                    value={formData.language || ''}
                                    onChange={(e) => setFormData({ ...formData, language: e.target.value })}
                                    placeholder="en"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label htmlFor="timezone">Timezone</Label>
                                <Input
                                    id="timezone"
                                    type="text"
                                    value={formData.timezone || ''}
                                    onChange={(e) => setFormData({ ...formData, timezone: e.target.value })}
                                    placeholder="UTC"
                                />
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

                                <div className="mt-4 border-t border-gray-200 pt-4">
                                    <div className="flex items-center space-x-2 mb-2">
                                        <Checkbox
                                            id="quiet_hours_enabled"
                                            checked={formData.preferences?.quiet_hours?.enabled ?? false}
                                            onCheckedChange={(checked) => setFormData({
                                                ...formData,
                                                preferences: {
                                                    ...formData.preferences,
                                                    quiet_hours: {
                                                        ...formData.preferences?.quiet_hours,
                                                        enabled: !!checked
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
                                                                ...formData.preferences?.quiet_hours,
                                                                start: e.target.value
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
                                                                ...formData.preferences?.quiet_hours,
                                                                end: e.target.value
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
                    <p className="text-gray-500 text-center py-8 text-sm">No users found for this application.</p>
                ) : (
                    <div className="overflow-x-auto">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>Email</TableHead>
                                    <TableHead>Language</TableHead>
                                    <TableHead>Actions</TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {users.map((user) => (
                                    <TableRow key={user.user_id}>
                                        <TableCell>{user.email || '-'}</TableCell>
                                        <TableCell>{user.language || 'en'}</TableCell>
                                        <TableCell>
                                            <button
                                                onClick={() => handleEditClick(user)}
                                                className="text-blue-600 hover:underline font-semibold mr-4"
                                            >
                                                Edit
                                            </button>
                                            <button
                                                onClick={() => handleDeleteUser(user.user_id)}
                                                className="text-red-600 hover:underline font-semibold"
                                            >
                                                Delete
                                            </button>
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </div>
                )}
            </CardContent>
        </Card>
    );
};

export default AppUsers;
