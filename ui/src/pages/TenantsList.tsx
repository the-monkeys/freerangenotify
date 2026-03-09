import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { tenantsAPI } from '../services/api';
import type { Tenant, CreateTenantRequest } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Spinner } from '../components/ui/spinner';
import { toast } from 'sonner';
import { extractErrorMessage } from '../lib/utils';
import { Building2 } from 'lucide-react';

const TenantsList: React.FC = () => {
  const navigate = useNavigate();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [formData, setFormData] = useState<CreateTenantRequest>({
    name: '',
  });
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    fetchTenants();
  }, []);

  const fetchTenants = async () => {
    setLoading(true);
    try {
      const data = await tenantsAPI.list();
      setTenants(Array.isArray(data) ? data : []);
    } catch (error) {
      console.error('Failed to fetch tenants:', error);
      toast.error(extractErrorMessage(error as Error, 'Failed to load organizations'));
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!formData.name.trim()) return;
    setCreating(true);
    try {
      const tenant = await tenantsAPI.create(formData);
      toast.success(`Organization "${tenant.name}" created`);
      setFormData({ name: '' });
      setShowForm(false);
      fetchTenants();
    } catch (err) {
      toast.error(extractErrorMessage(err as Error, 'Failed to create organization'));
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-3 mb-6">
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-foreground">Organizations</h1>
          <p className="text-muted-foreground mt-1 text-sm">Manage organizations and invite team members</p>
        </div>
        <Button
          onClick={() => setShowForm(!showForm)}
          variant={showForm ? 'outline' : 'default'}
        >
          {showForm ? 'Cancel' : '+ New Organization'}
        </Button>
      </div>

      {showForm && (
        <Card className="mb-6 border-t-2 border-t-border">
          <CardHeader>
            <CardTitle className="text-lg">Create New Organization</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleCreate}>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Organization Name</Label>
                  <Input
                    id="name"
                    type="text"
                    required
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    placeholder="Acme Inc."
                  />
                </div>
              </div>
              <div className="flex justify-end mt-6">
                <Button type="submit" disabled={creating}>
                  {creating ? 'Creating...' : 'Create Organization'}
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
          {(tenants ?? []).map((tenant) => (
            <Card
              key={tenant.id}
              className="cursor-pointer hover:shadow-sm transition-shadow"
              onClick={() => navigate(`/tenants/${tenant.id}`)}
            >
              <CardContent className="pt-6">
                <div className="flex items-start gap-3 mb-2">
                  <div className="rounded-lg bg-muted p-2">
                    <Building2 className="h-5 w-5 text-muted-foreground" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <h4 className="text-base font-semibold text-foreground truncate">{tenant.name}</h4>
                    <p className="text-xs text-muted-foreground mt-0.5">{tenant.slug}</p>
                  </div>
                </div>
                <div className="flex justify-between items-center border-t border-border pt-3 mt-2">
                  <span className="text-xs text-muted-foreground">Manage members &rarr;</span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {!loading && tenants.length === 0 && (
        <div className="text-center mt-20 text-muted-foreground">
          <div className="text-5xl mb-4">🏢</div>
          <h2 className="text-xl font-semibold mb-2">No organizations yet</h2>
          <p className="text-sm">Create an organization to collaborate with your team.</p>
          <Button className="mt-4" onClick={() => setShowForm(true)}>
            Create Organization
          </Button>
        </div>
      )}
    </div>
  );
};

export default TenantsList;
