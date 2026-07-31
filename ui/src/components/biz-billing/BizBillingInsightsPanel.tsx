import { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Loader2, RefreshCw } from 'lucide-react';
import { bizBillingAPI } from '../../services/api';
import type { BizStatement } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { type BizPanelProps, paisa } from './shared';

function MetricGrid({ data }: { data: Record<string, unknown> | null }) {
  if (!data) return <p className="text-sm text-muted-foreground">No data.</p>;
  const entries = Object.entries(data);
  if (!entries.length) return <p className="text-sm text-muted-foreground">No data.</p>;
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {entries.map(([k, v]) => (
        <div key={k} className="rounded-md border p-3">
          <div className="text-xs text-muted-foreground uppercase tracking-wide">{k.replace(/_/g, ' ')}</div>
          <div className="mt-1 text-lg font-semibold break-all">
            {typeof v === 'number' && (k.includes('paisa') || k.includes('mrr') || k.includes('arr') || k.includes('revenue'))
              ? `₹${paisa(v)}`
              : typeof v === 'object'
                ? JSON.stringify(v)
                : String(v)}
          </div>
        </div>
      ))}
    </div>
  );
}

export default function BizBillingInsightsPanel({ apiKey }: BizPanelProps) {
  const [loading, setLoading] = useState(false);
  const [revenue, setRevenue] = useState<Record<string, unknown> | null>(null);
  const [subs, setSubs] = useState<Record<string, unknown> | null>(null);
  const [invoices, setInvoices] = useState<Record<string, unknown> | null>(null);
  const [aging, setAging] = useState<Record<string, unknown> | null>(null);
  const [dunning, setDunning] = useState<Record<string, unknown> | null>(null);
  const [tax, setTax] = useState<Record<string, unknown> | null>(null);

  const [userId, setUserId] = useState('');
  const [statement, setStatement] = useState<BizStatement | null>(null);
  const [balanceAdj, setBalanceAdj] = useState(0);
  const [balanceReason, setBalanceReason] = useState('');
  const [portalUrl, setPortalUrl] = useState('');
  const [busy, setBusy] = useState(false);

  const loadAnalytics = async () => {
    setLoading(true);
    try {
      const [r, s, i, a, d, t] = await Promise.all([
        bizBillingAPI.analyticsRevenue(apiKey).catch(() => null),
        bizBillingAPI.analyticsSubscriptions(apiKey).catch(() => null),
        bizBillingAPI.analyticsInvoices(apiKey).catch(() => null),
        bizBillingAPI.analyticsAging(apiKey).catch(() => null),
        bizBillingAPI.analyticsDunning(apiKey).catch(() => null),
        bizBillingAPI.analyticsTaxSummary(apiKey).catch(() => null),
      ]);
      setRevenue(r);
      setSubs(s);
      setInvoices(i);
      setAging(a);
      setDunning(d);
      setTax(t);
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadAnalytics(); }, [apiKey]);

  const loadStatement = async () => {
    if (!userId.trim()) return;
    setBusy(true);
    try {
      const s = await bizBillingAPI.getUserStatement(apiKey, userId.trim());
      setStatement(s);
      setPortalUrl('');
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const adjustBalance = async () => {
    if (!userId.trim() || !balanceAdj) return;
    setBusy(true);
    try {
      const res = await bizBillingAPI.adjustUserBalance(apiKey, userId.trim(), {
        amount_paisa: Math.round(balanceAdj * 100),
        reason: balanceReason || undefined,
      });
      toast.success(`Balance: ₹${paisa(res.balance_paisa)}`);
      void loadStatement();
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const createPortal = async () => {
    if (!userId.trim()) return;
    setBusy(true);
    try {
      const res = await bizBillingAPI.createPortalLink(apiKey, userId.trim());
      setPortalUrl(res.url);
      toast.success('Portal link created');
    } catch (err) {
      toast.error(extractErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-4">
      <Tabs defaultValue="analytics">
        <TabsList>
          <TabsTrigger value="analytics">Analytics</TabsTrigger>
          <TabsTrigger value="customer">Customer</TabsTrigger>
        </TabsList>

        <TabsContent value="analytics" className="space-y-4 pt-4">
          <div className="flex justify-end">
            <Button variant="outline" size="sm" onClick={loadAnalytics} disabled={loading}>
              <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} /> Refresh
            </Button>
          </div>
          <Card>
            <CardHeader><CardTitle>Revenue</CardTitle><CardDescription>MRR / ARR and collected totals.</CardDescription></CardHeader>
            <CardContent><MetricGrid data={revenue} /></CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Subscriptions</CardTitle></CardHeader>
            <CardContent><MetricGrid data={subs} /></CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Invoices</CardTitle></CardHeader>
            <CardContent><MetricGrid data={invoices} /></CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Aging</CardTitle></CardHeader>
            <CardContent><MetricGrid data={aging} /></CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Dunning</CardTitle></CardHeader>
            <CardContent><MetricGrid data={dunning} /></CardContent>
          </Card>
          <Card>
            <CardHeader><CardTitle>Tax summary</CardTitle></CardHeader>
            <CardContent><MetricGrid data={tax} /></CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="customer" className="pt-4">
          <Card>
            <CardHeader>
              <CardTitle>Customer tools</CardTitle>
              <CardDescription>Statement, credit balance, and portal link.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex flex-wrap gap-2 items-end">
                <div className="space-y-1 grow min-w-[200px]">
                  <Label>User ID</Label>
                  <Input value={userId} onChange={e => setUserId(e.target.value)} placeholder="customer user id" />
                </div>
                <Button onClick={loadStatement} disabled={busy || !userId.trim()}>
                  {busy ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : null}
                  Load statement
                </Button>
                <Button variant="outline" onClick={createPortal} disabled={busy || !userId.trim()}>
                  Portal link
                </Button>
              </div>

              {portalUrl && (
                <div className="rounded-md border p-3 text-sm break-all">
                  <div className="text-xs text-muted-foreground mb-1">Portal URL</div>
                  <a className="text-primary underline" href={portalUrl} target="_blank" rel="noreferrer">{portalUrl}</a>
                </div>
              )}

              <div className="flex flex-wrap gap-2 items-end border-t pt-4">
                <div className="space-y-1">
                  <Label>Adjust balance (₹)</Label>
                  <Input type="number" value={balanceAdj} onChange={e => setBalanceAdj(+e.target.value)} />
                </div>
                <div className="space-y-1 grow min-w-[160px]">
                  <Label>Reason</Label>
                  <Input value={balanceReason} onChange={e => setBalanceReason(e.target.value)} />
                </div>
                <Button variant="secondary" onClick={adjustBalance} disabled={busy || !userId.trim() || !balanceAdj}>
                  Apply
                </Button>
              </div>

              {statement && (
                <div className="space-y-3 border-t pt-4">
                  <div className="grid gap-3 sm:grid-cols-4 text-sm">
                    <div>Billed: <strong>₹{paisa(statement.total_billed_paisa)}</strong></div>
                    <div>Paid: <strong>₹{paisa(statement.total_paid_paisa)}</strong></div>
                    <div>Owed: <strong>₹{paisa(statement.balance_owed_paisa)}</strong></div>
                    <div>Credit: <strong>₹{paisa(statement.credit_balance_paisa)}</strong></div>
                  </div>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Date</TableHead>
                        <TableHead>Type</TableHead>
                        <TableHead>Ref</TableHead>
                        <TableHead>Debit</TableHead>
                        <TableHead>Credit</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {(statement.entries || []).map((e, i) => (
                        <TableRow key={`${e.reference}-${i}`}>
                          <TableCell className="text-xs">{e.date ? new Date(e.date).toLocaleDateString() : '–'}</TableCell>
                          <TableCell>{e.type}</TableCell>
                          <TableCell className="text-xs">{e.reference}</TableCell>
                          <TableCell>{e.debit_paisa ? `₹${paisa(e.debit_paisa)}` : '–'}</TableCell>
                          <TableCell>{e.credit_paisa ? `₹${paisa(e.credit_paisa)}` : '–'}</TableCell>
                        </TableRow>
                      ))}
                      {(!statement.entries || statement.entries.length === 0) && (
                        <TableRow><TableCell colSpan={5} className="text-center py-6 text-muted-foreground">No entries.</TableCell></TableRow>
                      )}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
