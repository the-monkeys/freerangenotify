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
    <div className="p-8">
      <div className="flex justify-between items-center mb-8">
        <h1 className="text-3xl font-bold">Applications</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700"
        >
          {showForm ? 'Cancel' : 'New Application'}
        </button>
      </div>

      {showForm && (
        <form onSubmit={handleCreate} className="bg-white p-6 rounded-lg shadow mb-8">
          <div className="mb-4">
            <label className="block text-sm font-semibold mb-2">Name</label>
            <input
              type="text"
              required
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-600"
            />
          </div>
          <div className="mb-4">
            <label className="block text-sm font-semibold mb-2">Description</label>
            <textarea
              value={formData.description || ''}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-600"
              rows={4}
            />
          </div>
          <button
            type="submit"
            className="bg-green-600 text-white px-6 py-2 rounded-lg hover:bg-green-700"
          >
            Create Application
          </button>
        </form>
      )}

      {loading ? (
        <div className="text-center text-gray-600">Loading...</div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {apps.map((app) => (
            <div key={app.id} className="bg-white p-6 rounded-lg shadow-lg">
              <h3 className="text-xl font-semibold mb-2">{app.name}</h3>
              <p className="text-gray-600 mb-4">{app.description}</p>
              <div className="mb-4 p-3 bg-gray-100 rounded">
                <p className="text-xs font-mono text-gray-700 break-all">API Key: {app.apiKey}</p>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={() => handleRegenerateKey(app.id)}
                  className="flex-1 bg-yellow-500 text-white px-3 py-2 rounded hover:bg-yellow-600 text-sm"
                >
                  Regenerate Key
                </button>
                <button
                  onClick={() => handleDelete(app.id)}
                  className="flex-1 bg-red-600 text-white px-3 py-2 rounded hover:bg-red-700 text-sm"
                >
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && apps.length === 0 && (
        <div className="text-center text-gray-600 mt-8">
          <p>No applications found. Create one to get started!</p>
        </div>
      )}
    </div>
  );
};

export default AppsList;