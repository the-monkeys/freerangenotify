import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { applicationsAPI } from '../services/api';
import type { Application, CreateApplicationRequest } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Spinner } from '../components/ui/spinner';

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
    <div className="container mx-auto px-4 py-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Applications</h1>
          <p className="text-gray-500 mt-1 text-sm">Manage your notification applications and API keys</p>
        </div>
        <Button
          onClick={() => setShowForm(!showForm)}
          variant={showForm ? "outline" : "default"}
        >
          {showForm ? 'Cancel' : '+ New Application'}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6 border-t-2 border-t-blue-600">
          <CardHeader>
            <CardTitle className="text-lg">Create New Application</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate}>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="app_name">Application Name</Label>
                  <Input
                    id="app_name"
                    type="text"
                    required
                    value={formData.app_name}
                    onChange={(e) => setFormData({ ...formData, app_name: e.target.value })}
                    placeholder="My Awesome App"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="description">Description</Label>
                  <Input
                    id="description"
                    type="text"
                    value={formData.description || ''}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    placeholder="Short description..."
                  />
                </div>
              </div>
              <div className="flex justify-end mt-6">
                <Button type="submit">
                  Create Application
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}

      {loading ? (
        <div className="flex justify-center items-center py-12">
          <Spinner />
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {apps.map((app) => (
            <Card
              key={app.app_id}
              className="cursor-pointer hover:shadow-md transition-shadow"
              onClick={() => navigate(`/apps/${app.app_id}`)}
            >
              <CardContent className="pt-6">
                <div className="flex justify-between items-start mb-2">
                  <h4 className="text-base font-semibold text-blue-600">{app.app_name}</h4>
                  <Badge variant="outline" className="text-xs text-gray-400">ACTIVE</Badge>
                </div>
                <p className="text-gray-500 text-sm mb-6 min-h-10">
                  {app.description || 'No description provided.'}
                </p>

                <div className="flex justify-between items-center border-t border-gray-200 pt-3">
                  <span className="text-xs text-gray-400">ID: {app.app_id.substring(0, 8)}...</span>
                  <span className="text-blue-600 font-semibold text-xs">Details &rarr;</span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {!loading && apps.length === 0 && (
        <div className="text-center mt-20 text-gray-500">
          <div className="text-5xl mb-4">📦</div>
          <h2 className="text-xl font-semibold mb-2">No applications yet</h2>
          <p className="text-sm">Create your first application to get started.</p>
        </div>
      )}
    </div>
  );
};

export default AppsList;