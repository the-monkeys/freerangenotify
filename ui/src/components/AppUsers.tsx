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
                <h3 style={{ margin: 0 }}>Application Users</h3>
                <button
                    className="btn btn-primary"
                    onClick={() => setShowAddForm(!showAddForm)}
                >
                    {showAddForm ? 'Cancel' : 'Add User'}
                </button>
            </div>

            {showAddForm && (
                <form onSubmit={handleCreateUser} className="mb-8" style={{ background: '#1a202c', padding: '1.5rem', borderRadius: '0.5rem' }}>
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
                <p style={{ color: '#718096', textAlign: 'center', padding: '2rem' }}>No users found for this application.</p>
            ) : (
                <div style={{ overflowX: 'auto' }}>
                    <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                        <thead>
                            <tr style={{ borderBottom: '1px solid #2d3748', textAlign: 'left' }}>
                                <th style={{ padding: '1rem' }}>External ID</th>
                                <th style={{ padding: '1rem' }}>Email</th>
                                <th style={{ padding: '1rem' }}>Language</th>
                                <th style={{ padding: '1rem' }}>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map((user) => (
                                <tr key={user.user_id} style={{ borderBottom: '1px solid #1a202c' }}>
                                    <td style={{ padding: '1rem' }}>{user.external_user_id}</td>
                                    <td style={{ padding: '1rem' }}>{user.email || '-'}</td>
                                    <td style={{ padding: '1rem' }}>{user.language || 'en'}</td>
                                    <td style={{ padding: '1rem' }}>
                                        <button
                                            onClick={() => handleDeleteUser(user.user_id)}
                                            style={{ color: '#f56565', background: 'none', border: 'none', cursor: 'pointer' }}
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
