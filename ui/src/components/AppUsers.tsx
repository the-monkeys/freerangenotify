import React, { useEffect, useState } from 'react';
import { usersAPI } from '../services/api';
import type { User, CreateUserRequest } from '../types';

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
        } catch (error) {
            console.error('Failed to save user:', error);
            alert('Failed to save user');
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

    if (loading) return <div>Loading users...</div>;

    return (
        <div className="card">
            <div className="flex justify-between items-center mb-6">
                <h3 style={{ margin: 0, border: 'none' }}>Application Users</h3>
                <button
                    className="btn btn-primary"
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
                </button>
            </div>

            {showAddForm && (
                <form onSubmit={handleUpdateUser} className="mb-8" style={{ background: 'var(--azure-bg)', padding: '1.5rem', borderRadius: '2px', border: '1px solid var(--azure-border)' }}>
                    <div className="flex justify-between items-center mb-4">
                        <h4 style={{ margin: 0 }}>{editingUser ? 'Edit User' : 'Add New User'}</h4>
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">

                        <div className="form-group">
                            <label className="form-label">Custom User ID (Internal)</label>
                            <input
                                type="text"
                                className="form-input"
                                value={formData.user_id || ''}
                                onChange={(e) => setFormData({ ...formData, user_id: e.target.value })}
                                placeholder="e.g. EMP_001 (Optional)"
                                disabled={!!editingUser}
                                style={editingUser ? { backgroundColor: '#f3f2f1', cursor: 'not-allowed' } : {}}
                            />
                            {!editingUser && (
                                <p style={{ fontSize: '0.75rem', color: '#605e5c', marginTop: '0.2rem' }}>
                                    Leave empty to auto-generate a UUID.
                                </p>
                            )}
                        </div>
                        <div className="form-group">
                            <label className="form-label">Email</label>
                            <input
                                type="email"
                                className="form-input"
                                value={formData.email || ''}
                                onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                                placeholder="user@example.com"
                            />
                        </div>
                        <div className="form-group">
                            <label className="form-label">Language</label>
                            <input
                                type="text"
                                className="form-input"
                                value={formData.language || ''}
                                onChange={(e) => setFormData({ ...formData, language: e.target.value })}
                                placeholder="en"
                            />
                        </div>
                        <div className="form-group">
                            <label className="form-label">Timezone</label>
                            <input
                                type="text"
                                className="form-input"
                                value={formData.timezone || ''}
                                onChange={(e) => setFormData({ ...formData, timezone: e.target.value })}
                                placeholder="UTC"
                            />
                        </div>

                        <div className="form-group" style={{ gridColumn: '1 / -1' }}>
                            <label className="form-label" style={{ marginBottom: '0.5rem', display: 'block' }}>Preferences</label>
                            <div style={{ display: 'flex', gap: '1.5rem', flexWrap: 'wrap' }}>
                                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={formData.preferences?.email_enabled ?? true}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            preferences: { ...formData.preferences, email_enabled: e.target.checked }
                                        })}
                                    />
                                    <span>Email Enabled</span>
                                </label>
                                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={formData.preferences?.push_enabled ?? true}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            preferences: { ...formData.preferences, push_enabled: e.target.checked }
                                        })}
                                    />
                                    <span>Push Enabled</span>
                                </label>
                                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={formData.preferences?.sms_enabled ?? true}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            preferences: { ...formData.preferences, sms_enabled: e.target.checked }
                                        })}
                                    />
                                    <span>SMS Enabled</span>
                                </label>
                                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                                    <input
                                        type="checkbox"
                                        checked={formData.preferences?.dnd ?? false}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            preferences: { ...formData.preferences, dnd: e.target.checked }
                                        })}
                                    />
                                    <span>Do Not Disturb</span>
                                </label>
                            </div>

                            <div style={{ marginTop: '1rem', borderTop: '1px solid var(--azure-border)', paddingTop: '1rem' }}>
                                <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer', marginBottom: '0.5rem' }}>
                                    <input
                                        type="checkbox"
                                        checked={formData.preferences?.quiet_hours?.enabled ?? false}
                                        onChange={(e) => setFormData({
                                            ...formData,
                                            preferences: {
                                                ...formData.preferences,
                                                quiet_hours: {
                                                    ...formData.preferences?.quiet_hours,
                                                    enabled: e.target.checked
                                                }
                                            }
                                        })}
                                    />
                                    <span style={{ fontWeight: 600 }}>Quiet Hours</span>
                                </label>

                                {formData.preferences?.quiet_hours?.enabled && (
                                    <div style={{ display: 'flex', gap: '1rem', marginLeft: '1.5rem' }}>
                                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                                            <label style={{ fontSize: '0.75rem', marginBottom: '0.2rem' }}>Start Time</label>
                                            <input
                                                type="time"
                                                className="form-input"
                                                style={{ width: 'auto' }}
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
                                        <div style={{ display: 'flex', flexDirection: 'column' }}>
                                            <label style={{ fontSize: '0.75rem', marginBottom: '0.2rem' }}>End Time</label>
                                            <input
                                                type="time"
                                                className="form-input"
                                                style={{ width: 'auto' }}
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
                    <div className="flex justify-end mt-4">
                        <button type="submit" className="btn btn-primary">{editingUser ? 'Update User' : 'Create User'}</button>
                    </div>
                </form>
            )}

            {users.length === 0 ? (
                <p style={{ color: '#605e5c', textAlign: 'center', padding: '2rem', fontSize: '0.9rem' }}>No users found for this application.</p>
            ) : (
                <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid var(--azure-border)', textAlign: 'left' }}>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Email</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Language</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map((user) => (
                                <tr key={user.user_id} style={{ borderBottom: '1px solid var(--azure-border)' }}>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{user.email || '-'}</td>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{user.language || 'en'}</td>
                                    <td style={{ padding: '0.75rem' }}>
                                        <button
                                            onClick={() => handleEditClick(user)}
                                            style={{ color: '#0078d4', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 600, marginRight: '1rem' }}
                                        >
                                            Edit
                                        </button>
                                        <button
                                            onClick={() => handleDeleteUser(user.user_id)}
                                            style={{ color: '#a4262c', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 600 }}
                                        >
                                            Delete
                                        </button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
};

export default AppUsers;
