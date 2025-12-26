import React, { useEffect, useState } from 'react';
import { usersAPI } from '../services/api';
import type { User, CreateUserRequest } from '../types';

interface AppUsersProps {
    appId: string;
    apiKey: string;
}

const AppUsers: React.FC<AppUsersProps> = ({ appId, apiKey }) => {
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [formData, setFormData] = useState<CreateUserRequest>({
        external_user_id: '',
        email: '',
        timezone: 'UTC',
        language: 'en'
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

    const handleCreateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            await usersAPI.create(apiKey, formData);
            setShowAddForm(false);
            setFormData({
                external_user_id: '',
                email: '',
                timezone: 'UTC',
                language: 'en'
            });
            fetchUsers();
        } catch (error) {
            console.error('Failed to create user:', error);
            alert('Failed to create user');
        }
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
                    onClick={() => setShowAddForm(!showAddForm)}
                >
                    {showAddForm ? 'Cancel' : 'Add User'}
                </button>
            </div>

            {showAddForm && (
                <form onSubmit={handleCreateUser} className="mb-8" style={{ background: 'var(--azure-bg)', padding: '1.5rem', borderRadius: '2px', border: '1px solid var(--azure-border)' }}>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="form-group">
                            <label className="form-label">External User ID</label>
                            <input
                                type="text"
                                className="form-input"
                                value={formData.external_user_id}
                                onChange={(e) => setFormData({ ...formData, external_user_id: e.target.value })}
                                required
                                placeholder="e.g. user_123"
                            />
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
                    </div>
                    <div className="flex justify-end mt-4">
                        <button type="submit" className="btn btn-primary">Create User</button>
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
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>External ID</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Email</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Language</th>
                                <th style={{ padding: '0.75rem', color: '#605e5c' }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map((user) => (
                                <tr key={user.user_id} style={{ borderBottom: '1px solid var(--azure-border)' }}>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{user.external_user_id}</td>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{user.email || '-'}</td>
                                    <td style={{ padding: '0.75rem', color: '#323130' }}>{user.language || 'en'}</td>
                                    <td style={{ padding: '0.75rem' }}>
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
