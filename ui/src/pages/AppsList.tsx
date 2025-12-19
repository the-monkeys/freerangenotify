import React, { useEffect, useState } from 'react';
import { applicationsAPI } from '../services/api';
import type { Application, CreateApplicationRequest } from '../types';

const AppsList: React.FC = () => {
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState<CreateApplicationRequest>({
    name: '',
    description: '',
  });

  useEffect(() => {
    fetchApps();
  }, []);

  const fetchApps = async () => {
    setLoading(true);
    try {
      const data = await applicationsAPI.list();
      setApps(data);
    } catch (error) {
      console.error('Failed to fetch applications:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await applicationsAPI.create(formData);
      setFormData({ name: '', description: '' });
      setShowForm(false);
      fetchApps();
    } catch (error) {
      console.error('Failed to create application:', error);
    }
  };

  const handleDelete = async (id: string) => {
    if (window.confirm('Are you sure you want to delete this application?')) {
      try {
        await applicationsAPI.delete(id);
        fetchApps();
      } catch (error) {
        console.error('Failed to delete application:', error);
      }
    }
  };

  const handleRegenerateKey = async (id: string) => {
    try {
      const updated = await applicationsAPI.regenerateKey(id);
      setApps(apps.map(app => (app.id === id ? updated : app)));
    } catch (error) {
      console.error('Failed to regenerate key:', error);
    }
  };

  return (
    <div className="container">
      <div className="flex justify-between items-center mb-4">
        <h1 style={{ fontSize: '2rem' }}>Applications</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn btn-primary"
        >
          {showForm ? 'Cancel' : '+ New Application'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="card mb-4">
          <div className="form-group">
            <label className="form-label">Application Name</label>
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="form-input"
              placeholder="My Awesome App"
            />
          </div>
          <div className="form-group">
            <label className="form-label">Description</label>
            <textarea
              value={formData.description || ''}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="form-textarea"
              placeholder="Describe your application..."
            />
          </div>
          <button type="submit" className="btn btn-primary">
            Create Application
          </button>
        </form>
      )}

      {loading ? (
        <div className="spinner"></div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {apps.map((app) => (
            <div key={app.id} className="card">
              <h3 style={{ fontSize: '1.25rem', marginBottom: '0.5rem' }}>{app.name}</h3>
              <p style={{ color: '#718096', marginBottom: '1rem' }}>{app.description}</p>
              <div style={{
                padding: '0.75rem',
                background: '#f7fafc',
                borderRadius: '0.375rem',
                marginBottom: '1rem'
              }}>
                <p style={{
                  fontSize: '0.75rem',
                  fontFamily: 'monospace',
                  wordBreak: 'break-all',
                  color: '#4a5568'
                }}>
                  <strong>API Key:</strong><br />
                  {app.apiKey}
                </p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => handleRegenerateKey(app.id)}
                  className="btn btn-secondary"
                  style={{ flex: 1 }}
                >
                  🔄 Regenerate
                </button>
                <button
                  onClick={() => handleDelete(app.id)}
                  className="btn btn-danger"
                  style={{ flex: 1 }}
                >
                  🗑️ Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && apps.length === 0 && (
        <div className="text-center" style={{ marginTop: '3rem', color: '#718096' }}>
          <p style={{ fontSize: '1.125rem' }}>No applications found.</p>
          <p>Create one to get started! 🚀</p>
        </div>
      )}
    </div>
  );
};

export default AppsList;