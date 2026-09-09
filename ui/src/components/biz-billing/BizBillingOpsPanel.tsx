import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { bizBillingAPI } from '../../services/api';
import type { BizConnector, BizContract, BizExpense, BizUsageMeter } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '../ui/dialog';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { type BizPanelProps, paisa, statusBadge } from './shared';

export default function BizBillingOpsPanel({ apiKey }: BizPanelProps) {
  const [loading, setLoading] = useState(false);
  const [contracts, setContracts] = useState<BizContract[]>([]);
  const [meters, setMeters] = useState<BizUsageMeter[]>([]);
  const [expenses, setExpenses] = useState<BizExpense[]>([]);
  const [connectors, setConnectors] = useState<BizConnector[]>([]);
  const [acting, setActing] = useState<string | null>(null);

  const [ctOpen, setCtOpen] = useState(false);
  const [ctUser, setCtUser] = useState('');
  const [ctValue, setCtValue] = useState(0);
  const [ctStart, setCtStart] = useState('');
  const [ctEnd, setCtEnd] = useState('');
  const [ctTerms, setCtTerms] = useState('');

  const [mOpen, setMOpen] = useState(false);
  const [mName, setMName] = useState('');
  const [mUnit, setMUnit] = useState('');

  const [exOpen, setExOpen] = useState(false);
  const [exCat, setExCat] = useState('');
  const [exDesc, setExDesc] = useState('');
  const [exAmount, setExAmount] = useState(0);
  const [exVendor, setExVendor] = useState('');
  const [exBillable, setExBillable] = useState(false);
  const [exUser, setExUser] = useState('');

  const [cnOpen, setCnOpen] = useState(false);
  const [cnProvider, setCnProvider] = useState('razorpay');
  const [cnSecret, setCnSecret] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const [c, m, e, n] = await Promise.all([
        bizBillingAPI.listContracts(apiKey).catch(() => ({ data: [] as BizContract[] })),
        bizBillingAPI.listUsageMeters(apiKey).catch(() => [] as BizUsageMeter[]),
        bizBillingAPI.listExpenses(apiKey).catch(() => ({ data: [] as BizExpense[] })),
        bizBillingAPI.listConnectors(apiKey).catch(() => [] as BizConnector[]),
      ]);
      setContracts(c.data || []);
      setMeters(m || []);
      setExpenses(e.data || []);
      setConnectors(n || []);
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, [apiKey]);

  const act = async (id: string, label: string, fn: () => Promise<unknown>) => {
    setActing(id);
    try {
      await fn();
      toast.success(label);
      void load();
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setActing(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} /> Refresh
        </Button>
      </div>

      <Tabs defaultValue="contracts">
        <TabsList>
          <TabsTrigger value="contracts">Contracts</TabsTrigger>
          <TabsTrigger value="usage">Usage</TabsTrigger>
          <TabsTrigger value="expenses">Expenses</TabsTrigger>
          <TabsTrigger value="connectors">Connectors</TabsTrigger>
        </TabsList>

        <TabsContent value="contracts" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Contracts</CardTitle>
                <CardDescription>Service agreements with customers.</CardDescription>
              </div>
              <Dialog open={ctOpen} onOpenChange={setCtOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Contract</DialogTitle>
                    <DialogDescription className="sr-only">Create contract form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>User ID</Label><Input value={ctUser} onChange={e => setCtUser(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Value (₹)</Label><Input type="number" value={ctValue} onChange={e => setCtValue(+e.target.value)} /></div>
                    <div className="grid grid-cols-2 gap-2">
                      <div className="space-y-1"><Label>Start</Label><Input type="date" value={ctStart} onChange={e => setCtStart(e.target.value)} /></div>
                      <div className="space-y-1"><Label>End</Label><Input type="date" value={ctEnd} onChange={e => setCtEnd(e.target.value)} /></div>
                    </div>
                    <div className="space-y-1"><Label>Terms</Label><Input value={ctTerms} onChange={e => setCtTerms(e.target.value)} /></div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-ct', 'Contract created', async () => {
                      await bizBillingAPI.createContract(apiKey, {
                        user_id: ctUser,
                        value_paisa: Math.round(ctValue * 100),
                        start_date: new Date(ctStart).toISOString(),
                        end_date: new Date(ctEnd).toISOString(),
                        terms: ctTerms || undefined,
                      });
                      setCtOpen(false);
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>#</TableHead>
                    <TableHead>User</TableHead>
                    <TableHead>Value</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {contracts.map(c => (
                    <TableRow key={c.id}>
                      <TableCell className="font-medium">{c.contract_number || c.id.slice(0, 8)}</TableCell>
                      <TableCell className="text-xs">{c.user_id}</TableCell>
                      <TableCell>₹{paisa(c.value_paisa)}</TableCell>
                      <TableCell>{statusBadge(c.status)}</TableCell>
                      <TableCell className="text-right space-x-1">
                        {c.status === 'draft' && (
                          <Button size="sm" variant="outline" disabled={acting === c.id}
                            onClick={() => act(c.id, 'Sent', () => bizBillingAPI.sendContract(apiKey, c.id))}>Send</Button>
                        )}
                        {c.status === 'sent' && (
                          <Button size="sm" variant="outline" disabled={acting === c.id}
                            onClick={() => act(c.id, 'Accepted', () => bizBillingAPI.acceptContract(apiKey, c.id))}>Accept</Button>
                        )}
                        {c.status === 'active' && (
                          <>
                            <Button size="sm" variant="outline" disabled={acting === c.id}
                              onClick={() => act(c.id, 'Renewed', () => bizBillingAPI.renewContract(apiKey, c.id))}>Renew</Button>
                            <Button size="sm" variant="ghost" className="text-destructive" disabled={acting === c.id}
                              onClick={() => act(c.id, 'Terminated', () => bizBillingAPI.terminateContract(apiKey, c.id))}>Terminate</Button>
                          </>
                        )}
                        {acting === c.id && <Loader2 className="w-4 h-4 animate-spin inline-block" />}
                      </TableCell>
                    </TableRow>
                  ))}
                  {contracts.length === 0 && (
                    <TableRow><TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No contracts.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="usage" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Usage Meters</CardTitle>
                <CardDescription>Meters for usage-based billing.</CardDescription>
              </div>
              <Dialog open={mOpen} onOpenChange={setMOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Meter</DialogTitle>
                    <DialogDescription className="sr-only">Create usage meter form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>Name</Label><Input value={mName} onChange={e => setMName(e.target.value)} placeholder="API Calls" /></div>
                    <div className="space-y-1"><Label>Unit</Label><Input value={mUnit} onChange={e => setMUnit(e.target.value)} placeholder="calls" /></div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-m', 'Meter created', async () => {
                      await bizBillingAPI.createUsageMeter(apiKey, { name: mName, unit: mUnit || undefined, aggregation: 'sum' });
                      setMOpen(false);
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Unit</TableHead>
                    <TableHead>Aggregation</TableHead>
                    <TableHead>ID</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {meters.map(m => (
                    <TableRow key={m.id}>
                      <TableCell className="font-medium">{m.name}</TableCell>
                      <TableCell>{m.unit || '–'}</TableCell>
                      <TableCell>{m.aggregation}</TableCell>
                      <TableCell className="text-xs">{m.id}</TableCell>
                    </TableRow>
                  ))}
                  {meters.length === 0 && (
                    <TableRow><TableCell colSpan={4} className="text-center py-6 text-muted-foreground">No meters.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="expenses" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Expenses</CardTitle>
                <CardDescription>Track and optionally bill expenses to customers.</CardDescription>
              </div>
              <Dialog open={exOpen} onOpenChange={setExOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Expense</DialogTitle>
                    <DialogDescription className="sr-only">Create expense form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>Category</Label><Input value={exCat} onChange={e => setExCat(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Description</Label><Input value={exDesc} onChange={e => setExDesc(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Amount (₹)</Label><Input type="number" value={exAmount} onChange={e => setExAmount(+e.target.value)} /></div>
                    <div className="space-y-1"><Label>Vendor</Label><Input value={exVendor} onChange={e => setExVendor(e.target.value)} /></div>
                    <label className="flex items-center gap-2 text-sm">
                      <input type="checkbox" checked={exBillable} onChange={e => setExBillable(e.target.checked)} />
                      Billable
                    </label>
                    {exBillable && (
                      <div className="space-y-1"><Label>Billed to user ID</Label><Input value={exUser} onChange={e => setExUser(e.target.value)} /></div>
                    )}
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-ex', 'Expense created', async () => {
                      await bizBillingAPI.createExpense(apiKey, {
                        category: exCat,
                        description: exDesc || undefined,
                        amount_paisa: Math.round(exAmount * 100),
                        vendor: exVendor || undefined,
                        billable: exBillable,
                        billed_to_user: exBillable ? exUser : undefined,
                      });
                      setExOpen(false);
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Category</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Billable</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {expenses.map(e => (
                    <TableRow key={e.id}>
                      <TableCell className="font-medium">{e.category}</TableCell>
                      <TableCell>{e.description || '–'}</TableCell>
                      <TableCell>₹{paisa(e.amount_paisa)}</TableCell>
                      <TableCell>{e.billable ? (e.billed_to_user || 'yes') : 'no'}</TableCell>
                      <TableCell className="text-right">
                        <Button size="sm" variant="ghost" className="text-destructive" disabled={acting === e.id}
                          onClick={() => act(e.id, 'Deleted', () => bizBillingAPI.deleteExpense(apiKey, e.id))}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                  {expenses.length === 0 && (
                    <TableRow><TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No expenses.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="connectors" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Billing Connectors</CardTitle>
                <CardDescription>Third-party webhook connectors (Razorpay / Stripe / custom).</CardDescription>
              </div>
              <Dialog open={cnOpen} onOpenChange={setCnOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Connector</DialogTitle>
                    <DialogDescription className="sr-only">Create connector form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1">
                      <Label>Provider</Label>
                      <select className="w-full h-9 rounded-md border px-2 text-sm" value={cnProvider} onChange={e => setCnProvider(e.target.value)}>
                        <option value="razorpay">Razorpay</option>
                        <option value="stripe">Stripe</option>
                        <option value="custom">Custom</option>
                      </select>
                    </div>
                    <div className="space-y-1"><Label>Webhook secret</Label><Input value={cnSecret} onChange={e => setCnSecret(e.target.value)} /></div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-cn', 'Connector created', async () => {
                      await bizBillingAPI.createConnector(apiKey, {
                        provider: cnProvider,
                        webhook_secret: cnSecret,
                      });
                      setCnOpen(false);
                      setCnSecret('');
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Provider</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>ID</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {connectors.map(c => (
                    <TableRow key={c.id}>
                      <TableCell className="font-medium capitalize">{c.provider}</TableCell>
                      <TableCell>{statusBadge(c.active ? 'active' : 'void')}</TableCell>
                      <TableCell className="text-xs">{c.id}</TableCell>
                      <TableCell className="text-right space-x-1">
                        <Button size="sm" variant="outline" disabled={acting === c.id}
                          onClick={() => act(c.id, c.active ? 'Disabled' : 'Enabled', () =>
                            bizBillingAPI.updateConnector(apiKey, c.id, { active: !c.active }))}>
                          {c.active ? 'Disable' : 'Enable'}
                        </Button>
                        <Button size="sm" variant="ghost" className="text-destructive" disabled={acting === c.id}
                          onClick={() => act(c.id, 'Deleted', () => bizBillingAPI.deleteConnector(apiKey, c.id))}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                  {connectors.length === 0 && (
                    <TableRow><TableCell colSpan={4} className="text-center py-6 text-muted-foreground">No connectors.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
