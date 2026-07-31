import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react';
import { bizBillingAPI } from '../../services/api';
import type { BizCoupon, BizCreditNote, BizEstimate, BizRetainer } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '../ui/dialog';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { type BizPanelProps, paisa, statusBadge } from './shared';

export default function BizBillingSalesPanel({ apiKey }: BizPanelProps) {
  const [loading, setLoading] = useState(false);
  const [estimates, setEstimates] = useState<BizEstimate[]>([]);
  const [coupons, setCoupons] = useState<BizCoupon[]>([]);
  const [credits, setCredits] = useState<BizCreditNote[]>([]);
  const [retainers, setRetainers] = useState<BizRetainer[]>([]);
  const [acting, setActing] = useState<string | null>(null);

  const [estOpen, setEstOpen] = useState(false);
  const [estUser, setEstUser] = useState('');
  const [estDesc, setEstDesc] = useState('');
  const [estQty, setEstQty] = useState(1);
  const [estPrice, setEstPrice] = useState(0);
  const [estTax, setEstTax] = useState(18);

  const [cpOpen, setCpOpen] = useState(false);
  const [cpCode, setCpCode] = useState('');
  const [cpType, setCpType] = useState<'percentage' | 'fixed'>('percentage');
  const [cpValue, setCpValue] = useState(10);

  const [cnOpen, setCnOpen] = useState(false);
  const [cnUser, setCnUser] = useState('');
  const [cnAmount, setCnAmount] = useState(0);
  const [cnReason, setCnReason] = useState('');
  const [cnInvoice, setCnInvoice] = useState('');

  const [rtOpen, setRtOpen] = useState(false);
  const [rtUser, setRtUser] = useState('');
  const [rtAmount, setRtAmount] = useState(0);
  const [rtNotes, setRtNotes] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const [e, c, n, r] = await Promise.all([
        bizBillingAPI.listEstimates(apiKey).catch(() => ({ data: [] as BizEstimate[] })),
        bizBillingAPI.listCoupons(apiKey).catch(() => ({ data: [] as BizCoupon[] })),
        bizBillingAPI.listCreditNotes(apiKey).catch(() => ({ data: [] as BizCreditNote[] })),
        bizBillingAPI.listRetainers(apiKey).catch(() => ({ data: [] as BizRetainer[] })),
      ]);
      setEstimates(e.data || []);
      setCoupons(c.data || []);
      setCredits(n.data || []);
      setRetainers(r.data || []);
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

      <Tabs defaultValue="estimates">
        <TabsList>
          <TabsTrigger value="estimates">Estimates</TabsTrigger>
          <TabsTrigger value="coupons">Coupons</TabsTrigger>
          <TabsTrigger value="credits">Credit Notes</TabsTrigger>
          <TabsTrigger value="retainers">Retainers</TabsTrigger>
        </TabsList>

        <TabsContent value="estimates" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Estimates</CardTitle>
                <CardDescription>Quotes that convert to invoices.</CardDescription>
              </div>
              <Dialog open={estOpen} onOpenChange={setEstOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Estimate</DialogTitle>
                    <DialogDescription className="sr-only">Create estimate form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>User ID</Label><Input value={estUser} onChange={e => setEstUser(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Description</Label><Input value={estDesc} onChange={e => setEstDesc(e.target.value)} /></div>
                    <div className="grid grid-cols-3 gap-2">
                      <div className="space-y-1"><Label>Qty</Label><Input type="number" value={estQty} onChange={e => setEstQty(+e.target.value)} /></div>
                      <div className="space-y-1"><Label>Unit (₹)</Label><Input type="number" value={estPrice} onChange={e => setEstPrice(+e.target.value)} /></div>
                      <div className="space-y-1"><Label>Tax %</Label><Input type="number" value={estTax} onChange={e => setEstTax(+e.target.value)} /></div>
                    </div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-est', 'Estimate created', async () => {
                      await bizBillingAPI.createEstimate(apiKey, {
                        user_id: estUser,
                        currency: 'INR',
                        line_items: [{
                          description: estDesc,
                          quantity: estQty,
                          unit_price: Math.round(estPrice * 100),
                          tax_rate: estTax,
                        }],
                      });
                      setEstOpen(false);
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
                    <TableHead>Total</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {estimates.map(e => (
                    <TableRow key={e.id}>
                      <TableCell className="font-medium">{e.estimate_number || e.id.slice(0, 8)}</TableCell>
                      <TableCell className="text-xs">{e.user_id}</TableCell>
                      <TableCell>₹{paisa(e.total_paisa)} {e.currency}</TableCell>
                      <TableCell>{statusBadge(e.status)}</TableCell>
                      <TableCell className="text-right space-x-1">
                        {e.status === 'draft' && (
                          <Button size="sm" variant="outline" disabled={acting === e.id}
                            onClick={() => act(e.id, 'Sent', () => bizBillingAPI.sendEstimate(apiKey, e.id))}>Send</Button>
                        )}
                        {e.status === 'sent' && (
                          <>
                            <Button size="sm" variant="outline" disabled={acting === e.id}
                              onClick={() => act(e.id, 'Accepted', () => bizBillingAPI.acceptEstimate(apiKey, e.id))}>Accept</Button>
                            <Button size="sm" variant="outline" disabled={acting === e.id}
                              onClick={() => act(e.id, 'Rejected', () => bizBillingAPI.rejectEstimate(apiKey, e.id))}>Reject</Button>
                          </>
                        )}
                        {(e.status === 'accepted' || e.status === 'sent') && (
                          <Button size="sm" disabled={acting === e.id}
                            onClick={() => act(e.id, 'Converted', () => bizBillingAPI.convertEstimate(apiKey, e.id))}>Convert</Button>
                        )}
                        {e.status === 'draft' && (
                          <Button size="sm" variant="ghost" className="text-destructive" disabled={acting === e.id}
                            onClick={() => act(e.id, 'Deleted', () => bizBillingAPI.deleteEstimate(apiKey, e.id))}>
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        )}
                        {acting === e.id && <Loader2 className="w-4 h-4 animate-spin inline-block" />}
                      </TableCell>
                    </TableRow>
                  ))}
                  {estimates.length === 0 && (
                    <TableRow><TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No estimates.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="coupons" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Coupons</CardTitle>
                <CardDescription>Discount codes for invoices and plans.</CardDescription>
              </div>
              <Dialog open={cpOpen} onOpenChange={setCpOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Coupon</DialogTitle>
                    <DialogDescription className="sr-only">Create coupon form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>Code</Label><Input value={cpCode} onChange={e => setCpCode(e.target.value)} /></div>
                    <div className="grid grid-cols-2 gap-2">
                      <div className="space-y-1">
                        <Label>Type</Label>
                        <select className="w-full h-9 rounded-md border px-2 text-sm" value={cpType} onChange={e => setCpType(e.target.value as 'percentage' | 'fixed')}>
                          <option value="percentage">Percentage</option>
                          <option value="fixed">Fixed (₹)</option>
                        </select>
                      </div>
                      <div className="space-y-1">
                        <Label>Value</Label>
                        <Input type="number" value={cpValue} onChange={e => setCpValue(+e.target.value)} />
                      </div>
                    </div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-cp', 'Coupon created', async () => {
                      await bizBillingAPI.createCoupon(apiKey, {
                        code: cpCode,
                        discount_type: cpType,
                        discount_value: cpType === 'fixed' ? Math.round(cpValue * 100) : cpValue,
                      });
                      setCpOpen(false);
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Code</TableHead>
                    <TableHead>Discount</TableHead>
                    <TableHead>Uses</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {coupons.map(c => (
                    <TableRow key={c.id}>
                      <TableCell className="font-medium">{c.code}</TableCell>
                      <TableCell>
                        {c.discount_type === 'percentage' ? `${c.discount_value}%` : `₹${paisa(c.discount_value)}`}
                      </TableCell>
                      <TableCell>{c.current_uses}{c.max_uses ? ` / ${c.max_uses}` : ''}</TableCell>
                      <TableCell>{statusBadge(c.active ? 'active' : 'void')}</TableCell>
                      <TableCell className="text-right space-x-1">
                        <Button size="sm" variant="outline" disabled={acting === c.id}
                          onClick={() => act(c.id, c.active ? 'Disabled' : 'Enabled', () =>
                            bizBillingAPI.updateCoupon(apiKey, c.id, { active: !c.active }))}>
                          {c.active ? 'Disable' : 'Enable'}
                        </Button>
                        <Button size="sm" variant="ghost" className="text-destructive" disabled={acting === c.id}
                          onClick={() => act(c.id, 'Deleted', () => bizBillingAPI.deleteCoupon(apiKey, c.id))}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                  {coupons.length === 0 && (
                    <TableRow><TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No coupons.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="credits" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Credit Notes</CardTitle>
                <CardDescription>Credits issued to customers.</CardDescription>
              </div>
              <Dialog open={cnOpen} onOpenChange={setCnOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Credit Note</DialogTitle>
                    <DialogDescription className="sr-only">Create credit note form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>User ID</Label><Input value={cnUser} onChange={e => setCnUser(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Invoice ID (optional)</Label><Input value={cnInvoice} onChange={e => setCnInvoice(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Amount (₹)</Label><Input type="number" value={cnAmount} onChange={e => setCnAmount(+e.target.value)} /></div>
                    <div className="space-y-1"><Label>Reason</Label><Input value={cnReason} onChange={e => setCnReason(e.target.value)} /></div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-cn', 'Credit note created', async () => {
                      await bizBillingAPI.createCreditNote(apiKey, {
                        user_id: cnUser,
                        invoice_id: cnInvoice || undefined,
                        amount_paisa: Math.round(cnAmount * 100),
                        reason: cnReason || undefined,
                      });
                      setCnOpen(false);
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
                    <TableHead>Amount</TableHead>
                    <TableHead>Balance</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {credits.map(n => (
                    <TableRow key={n.id}>
                      <TableCell className="font-medium">{n.credit_note_number || n.id.slice(0, 8)}</TableCell>
                      <TableCell className="text-xs">{n.user_id}</TableCell>
                      <TableCell>₹{paisa(n.amount_paisa)}</TableCell>
                      <TableCell>₹{paisa(n.balance_paisa)}</TableCell>
                      <TableCell>{statusBadge(n.status)}</TableCell>
                      <TableCell className="text-right">
                        {n.status === 'open' && (
                          <Button size="sm" variant="outline" disabled={acting === n.id}
                            onClick={() => act(n.id, 'Refunded', () => bizBillingAPI.refundCreditNote(apiKey, n.id))}>
                            Refund
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                  {credits.length === 0 && (
                    <TableRow><TableCell colSpan={6} className="text-center py-6 text-muted-foreground">No credit notes.</TableCell></TableRow>
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="retainers" className="pt-4">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <div>
                <CardTitle>Retainers</CardTitle>
                <CardDescription>Advance deposits drawn down against invoices.</CardDescription>
              </div>
              <Dialog open={rtOpen} onOpenChange={setRtOpen}>
                <DialogTrigger asChild>
                  <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New</Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Create Retainer</DialogTitle>
                    <DialogDescription className="sr-only">Create retainer form</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-3 py-2">
                    <div className="space-y-1"><Label>User ID</Label><Input value={rtUser} onChange={e => setRtUser(e.target.value)} /></div>
                    <div className="space-y-1"><Label>Amount (₹)</Label><Input type="number" value={rtAmount} onChange={e => setRtAmount(+e.target.value)} /></div>
                    <div className="space-y-1"><Label>Notes</Label><Input value={rtNotes} onChange={e => setRtNotes(e.target.value)} /></div>
                  </div>
                  <DialogFooter>
                    <Button onClick={() => act('create-rt', 'Retainer created', async () => {
                      await bizBillingAPI.createRetainer(apiKey, {
                        user_id: rtUser,
                        amount_paisa: Math.round(rtAmount * 100),
                        notes: rtNotes || undefined,
                      });
                      setRtOpen(false);
                    })}>Create</Button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>User</TableHead>
                    <TableHead>Amount</TableHead>
                    <TableHead>Balance</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {retainers.map(r => (
                    <TableRow key={r.id}>
                      <TableCell className="text-xs font-medium">{r.id.slice(0, 8)}</TableCell>
                      <TableCell className="text-xs">{r.user_id}</TableCell>
                      <TableCell>₹{paisa(r.amount_paisa)}</TableCell>
                      <TableCell>₹{paisa(r.balance_paisa)}</TableCell>
                      <TableCell>{statusBadge(r.status)}</TableCell>
                      <TableCell className="text-right">
                        {r.status === 'open' && (
                          <Button size="sm" variant="outline" disabled={acting === r.id}
                            onClick={() => act(r.id, 'Marked paid', () => bizBillingAPI.markRetainerPaid(apiKey, r.id))}>
                            Mark paid
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                  {retainers.length === 0 && (
                    <TableRow><TableCell colSpan={6} className="text-center py-6 text-muted-foreground">No retainers.</TableCell></TableRow>
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
