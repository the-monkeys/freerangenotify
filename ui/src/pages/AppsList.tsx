import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { applicationsAPI } from '../services/api';
import type { Application, CreateApplicationRequest } from '../types';

const AppsList: React.FC = () => {
  const navigate = useNavigate();
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState<CreateApplicationRequest>({
    app_name: '',
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
      setFormData({ app_name: '', description: '' });
      setShowForm(false);
      fetchApps();
    } catch (error) {
      console.error('Failed to create application:', error);
    }
  };

  return (
    <div className="container">
      <div className="flex justify-between items-center mb-4">
        <div>
          <h1 style={{ fontSize: '2rem', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)', WebkitBackgroundClip: 'text', WebkitTextFillColor: 'transparent' }}>Applications</h1>
          <p style={{ color: '#718096', marginTop: '0.5rem' }}>Manage your notification apps and keys</p>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="btn btn-primary"
        >
          {showForm ? 'Cancel' : '+ New Application'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="card mb-4" style={{ borderLeft: '4px solid #667eea' }}>
          <h3 className="mb-4">Create New Application</h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="form-group">
              <label className="form-label">Application Name</label>
              <input
                type="text"
                required
                value={formData.app_name}
                onChange={(e) => setFormData({ ...formData, app_name: e.target.value })}
                className="form-input"
                placeholder="My Awesome App"
              />
            </div>
            <div className="form-group">
              <label className="form-label">Description</label>
              <input
                type="text"
                value={formData.description || ''}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                className="form-input"
                placeholder="Short description..."
              />
            </div>
          </div>
          <div className="flex justify-end mt-4">
            <button type="submit" className="btn btn-primary">
              Create Application
            </button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="center"><div className="spinner"></div></div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {apps.map((app) => (
            <div
              key={app.app_id}
              className="card"
              style={{ cursor: 'pointer', position: 'relative', overflow: 'hidden' }}
              onClick={() => navigate(`/apps/${app.app_id}`)}
            >
              <div style={{ position: 'absolute', top: 0, left: 0, width: '100%', height: '4px', background: 'linear-gradient(90deg, #667eea, #764ba2)' }}></div>
              <h3 style={{ fontSize: '1.25rem', marginBottom: '0.5rem', marginTop: '0.5rem' }}>{app.app_name}</h3>
              <p style={{ color: '#718096', marginBottom: '1.5rem', minHeight: '3rem' }}>
                {app.description || 'No description provided.'}
              </p>

              <div className="flex justify-between items-center" style={{ borderTop: '1px solid #e2e8f0', paddingTop: '1rem' }}>
                <span style={{ fontSize: '0.875rem', color: '#a0aec0' }}>ID: {app.app_id.substring(0, 8)}...</span>
                <span style={{ color: '#667eea', fontWeight: 500, fontSize: '0.875rem' }}>View Details &rarr;</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && apps.length === 0 && (
        <div className="text-center" style={{ marginTop: '5rem', color: '#718096' }}>
          <div style={{ fontSize: '4rem', marginBottom: '1rem' }}>🚀</div>
          <h2 style={{ marginBottom: '0.5rem' }}>No applications yet</h2>
          <p>Create your first application to get started.</p>
        </div>
      )}
    </div>
  );
};

export default AppsList;