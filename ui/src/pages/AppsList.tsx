import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { applicationsAPI, tenantsAPI } from '../services/api';
import type {  CreateApplicationRequest, Tenant } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Badge } from '../components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../components/ui/select';
import { Spinner } from '../components/ui/spinner';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';

import { useApps } from '../contexts/AppsContext';

const AppsList: React.FC = () => {
  const navigate = useNavigate();
  const { apps, loading, refreshApps } = useApps();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState<CreateApplicationRequest>({
    app_name: '',
    description: '',
  });

  useEffect(() => {
    if (showForm) {
      tenantsAPI.list().then((d) => setTenants(Array.isArray(d) ? d : [])).catch(() => setTenants([]));
    }
  }, [showForm]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await applicationsAPI.create(formData);
      setFormData({ app_name: '', description: '' });
      setShowForm(false);
      refreshApps();
    } catch (error) {
      toast.error(extractErrorMessage(error as Error, 'Failed to create application'));
    }
  };

  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-6">
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-foreground">Applications</h1>
          <p className="text-muted-foreground mt-1 text-sm">Manage your notification applications and API keys</p>
        </div>
        <Button
          onClick={() => setShowForm(!showForm)}
          variant={showForm ? "outline" : "default"}
        >
          {showForm ? 'Cancel' : '+ New Application'}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6 border-t-2 border-t-border">
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
                {tenants.length > 0 && (
                  <div className="space-y-2 md:col-span-2">
                    <Label htmlFor="tenant_id">Organization (optional)</Label>
                    <Select
                      value={formData.tenant_id || 'none'}
                      onValueChange={(v) => setFormData({ ...formData, tenant_id: v === 'none' ? undefined : v })}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Personal workspace" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">Personal workspace</SelectItem>
                        {(tenants ?? []).map((t) => (
                          <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
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
          {(apps ?? []).map((app) => (
            <Card
              key={app.app_id}
              className="cursor-pointer hover:shadow-sm transition-shadow"
              onClick={() => navigate(`/apps/${app.app_id}`)}
            >
              <CardContent className="pt-6">
                <div className="flex justify-between items-start mb-2">
                  <h4 className="text-base font-semibold text-foreground">{app.app_name}</h4>
                  <Badge variant="outline" className="text-xs text-muted-foreground">ACTIVE</Badge>
                </div>
                <p className="text-muted-foreground text-sm mb-6 min-h-10">
                  {app.description || 'No description provided.'}
                </p>

                <div className="flex justify-between items-center border-t border-border pt-3">
                  <span className="text-xs text-muted-foreground">ID: {app.app_id.substring(0, 8)}...</span>
                  <span className="text-accent font-semibold text-xs">Details &rarr;</span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {!loading && apps.length === 0 && (
        <div className="text-center mt-20 text-muted-foreground">
          <div className="text-5xl mb-4">📦</div>
          <h2 className="text-xl font-semibold mb-2">No applications yet</h2>
          <p className="text-sm">Create your first application to get started.</p>
        </div>
      )}
    </div>
  );
};

export default AppsList;