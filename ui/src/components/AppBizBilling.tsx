import React, { useEffect, useState } from 'react';
import { toast } from 'sonner';
import { Loader2, Plus, RefreshCw, Edit, Trash2, Pause, Play, XCircle, FileText, Ban } from 'lucide-react';
import { bizBillingAPI } from '../services/api';
import type { BizProduct, BizPlan, BizPlanAddon, BizSubscription, BizInvoice, BizPayment } from '../types';
import { extractErrorMessage } from '../lib/utils';
import { Button } from './ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from './ui/card';
import { Dialog, DialogContent, DialogHeader, DialogDescription, DialogTitle, DialogFooter, DialogTrigger } from './ui/dialog';
import { Input } from './ui/input';
import { Label } from './ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from './ui/table';
import { Badge } from './ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from './ui/tabs';
import { Textarea } from './ui/textarea';
import BizBillingSalesPanel from './biz-billing/BizBillingSalesPanel';
import BizBillingOpsPanel from './biz-billing/BizBillingOpsPanel';
import BizBillingInsightsPanel from './biz-billing/BizBillingInsightsPanel';

interface AppBizBillingProps {
    apiKey: string;
    appId: string;
}

const AppBizBilling: React.FC<AppBizBillingProps> = ({ apiKey, appId }) => {
    const [products, setProducts] = useState<BizProduct[]>([]);
    const [plans, setPlans] = useState<BizPlan[]>([]);
    const [subscriptions, setSubscriptions] = useState<BizSubscription[]>([]);
    const [invoices, setInvoices] = useState<BizInvoice[]>([]);
    const [payments, setPayments] = useState<BizPayment[]>([]);
    const [loading, setLoading] = useState(false);

    // Form states
    const [prodName, setProdName] = useState('');
    const [prodDesc, setProdDesc] = useState('');
    // Form states for Create Plan
    const [planName, setPlanName] = useState('');
    const [planDesc, setPlanDesc] = useState('');
    const [planPrice, setPlanPrice] = useState(0);
    const [planCycle, setPlanCycle] = useState('monthly');
    const [planProduct, setPlanProduct] = useState('');
    const [isProductOpen, setIsProductOpen] = useState(false);
    const [isPlanOpen, setIsPlanOpen] = useState(false);

    // Edit Plan state
    const [editingPlan, setEditingPlan] = useState<BizPlan | null>(null);
    const [editPlanName, setEditPlanName] = useState('');
    const [editPlanDesc, setEditPlanDesc] = useState('');
    const [editPlanPrice, setEditPlanPrice] = useState(0);
    const [editPlanCycle, setEditPlanCycle] = useState('monthly');
    const [isEditPlanOpen, setIsEditPlanOpen] = useState(false);
    const [deletingPlanId, setDeletingPlanId] = useState<string | null>(null);

    // Addons State
    const [managingAddonsPlan, setManagingAddonsPlan] = useState<BizPlan | null>(null);
    const [addons, setAddons] = useState<BizPlanAddon[]>([]);
    const [addonName, setAddonName] = useState('');
    const [addonPrice, setAddonPrice] = useState(0);
    const [addonCycle, setAddonCycle] = useState('monthly');
    const [isAddonsOpen, setIsAddonsOpen] = useState(false);

    // Edit product state
    const [editingProduct, setEditingProduct] = useState<BizProduct | null>(null);
    const [editProdName, setEditProdName] = useState('');
    const [editProdDesc, setEditProdDesc] = useState('');
    const [editProdActive, setEditProdActive] = useState(true);
    const [isEditProductOpen, setIsEditProductOpen] = useState(false);
    const [deletingProductId, setDeletingProductId] = useState<string | null>(null);

    // simple json text area for config
    const [configText, setConfigText] = useState('{\n  "stripe": {},\n  "razorpay": {}\n}');

    // Subscription action states
    const [actingSubId, setActingSubId] = useState<string | null>(null);
    const [actingInvoiceId, setActingInvoiceId] = useState<string | null>(null);

    // Create subscription dialog
    const [isCreateSubOpen, setIsCreateSubOpen] = useState(false);
    const [newSubUserId, setNewSubUserId] = useState('');
    const [newSubPlanId, setNewSubPlanId] = useState('');
    const [newSubQty, setNewSubQty] = useState(1);
    const [newSubTrial, setNewSubTrial] = useState(0);

    // Create invoice dialog
    const [isCreateInvOpen, setIsCreateInvOpen] = useState(false);
    const [newInvUserId, setNewInvUserId] = useState('');
    const [newInvCurrency, setNewInvCurrency] = useState('INR');
    const [newInvDesc, setNewInvDesc] = useState('');
    const [newInvQty, setNewInvQty] = useState(1);
    const [newInvUnitPrice, setNewInvUnitPrice] = useState(0);
    const [newInvTaxRate, setNewInvTaxRate] = useState(18);
    const [newInvTerms, setNewInvTerms] = useState('net30');

    const load = async () => {
        setLoading(true);
        try {
            const [prodRes, planRes, subRes, invRes, payRes, confRes] = await Promise.all([
                bizBillingAPI.listProducts(apiKey).catch(() => [] as BizProduct[]),
                bizBillingAPI.listPlans(apiKey).catch(() => [] as BizPlan[]),
                bizBillingAPI.listSubscriptions(apiKey).catch(() => ({ data: [] as BizSubscription[], total: 0 })),
                bizBillingAPI.listInvoices(apiKey).catch(() => ({ data: [] as BizInvoice[], total: 0 })),
                bizBillingAPI.listPayments(apiKey).catch(() => ({ data: [] as BizPayment[], total: 0 })),
                bizBillingAPI.getConfig(apiKey, 'payment_terms').catch(() => null)
            ]);
            setProducts(prodRes as BizProduct[]);
            setPlans(planRes as BizPlan[]);
            setSubscriptions(subRes.data || []);
            setInvoices(invRes.data || []);
            setPayments(payRes.data || []);
            if (confRes && confRes.data) {
                setConfigText(JSON.stringify(confRes.data, null, 2));
            }
        } catch (e) {
            toast.error('Failed to load billing data: ' + extractErrorMessage(e));
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void load();
    }, [apiKey]);

    const handleSaveConfig = async () => {
        try {
            const parsed = JSON.parse(configText);
            await bizBillingAPI.updateConfig(apiKey, 'payment_terms', { data: parsed, config_type: 'payment_terms', app_id: appId });
            toast.success('Config saved');
        } catch (e) {
            toast.error('Invalid JSON or save failed');
        }
    };

    const handleCreateProduct = async () => {
        try {
            await bizBillingAPI.createProduct(apiKey, {
                name: prodName,
                description: prodDesc,
                active: true,
                app_id: appId,
            });
            toast.success('Product created');
            setIsProductOpen(false);
            void load();
        } catch (e) {
            toast.error('Failed to create product');
        }
    };

    const handleEditProduct = (p: BizProduct) => {
        setEditingProduct(p);
        setEditProdName(p.name);
        setEditProdDesc(p.description || '');
        setEditProdActive(p.active);
        setIsEditProductOpen(true);
    };

    const handleUpdateProduct = async () => {
        if (!editingProduct?.id) return;
        try {
            await bizBillingAPI.updateProduct(apiKey, editingProduct.id, {
                name: editProdName,
                description: editProdDesc,
                active: editProdActive,
            });
            toast.success('Product updated');
            setIsEditProductOpen(false);
            setEditingProduct(null);
            void load();
        } catch (e) {
            toast.error('Failed to update product: ' + extractErrorMessage(e));
        }
    };

    const handleDeleteProduct = async () => {
        if (!deletingProductId) return;
        try {
            await bizBillingAPI.deleteProduct(apiKey, deletingProductId);
            toast.success('Product archived');
            setDeletingProductId(null);
            void load();
        } catch (e) {
            toast.error('Failed to delete product: ' + extractErrorMessage(e));
        }
    };

    const handleCreatePlan = async () => {
        try {
            await bizBillingAPI.createPlan(apiKey, {
                name: planName,
                description: planDesc,
                amount_paisa: Math.round(planPrice * 100),
                billing_cycle: planCycle,
                product_id: planProduct,
                currency: 'USD',
                active: true,
                app_id: appId,
            });
            toast.success('Plan created');
            setIsPlanOpen(false);
            setPlanName('');
            setPlanDesc('');
            setPlanPrice(0);
            void load();
        } catch (e) {
            toast.error('Failed to create plan: ' + extractErrorMessage(e));
        }
    };

    const handleEditPlan = (p: BizPlan) => {
        setEditingPlan(p);
        setEditPlanName(p.name);
        setEditPlanDesc(p.description || '');
        setEditPlanPrice((p.amount_paisa || 0) / 100);
        setEditPlanCycle(p.billing_cycle || 'monthly');
        setIsEditPlanOpen(true);
    };

    const handleUpdatePlan = async () => {
        if (!editingPlan?.id) return;
        try {
            await bizBillingAPI.updatePlan(apiKey, editingPlan.id, {
                name: editPlanName,
                description: editPlanDesc,
                amount_paisa: Math.round(editPlanPrice * 100),
                billing_cycle: editPlanCycle,
            });
            toast.success('Plan updated');
            setIsEditPlanOpen(false);
            setEditingPlan(null);
            void load();
        } catch (e) {
            toast.error('Failed to update plan: ' + extractErrorMessage(e));
        }
    };

    const handleDeletePlan = async () => {
        if (!deletingPlanId) return;
        try {
            await bizBillingAPI.deletePlan(apiKey, deletingPlanId);
            toast.success('Plan archived');
            setDeletingPlanId(null);
            void load();
        } catch (e) {
            toast.error('Failed to archive plan: ' + extractErrorMessage(e));
        }
    };

    const handleManageAddons = async (p: BizPlan) => {
        if (!p.id) return;
        setManagingAddonsPlan(p);
        setIsAddonsOpen(true);
        try {
            const data = await bizBillingAPI.listPlanAddons(apiKey, p.id);
            setAddons(data || []);
        } catch (e) {
            toast.error('Failed to load addons: ' + extractErrorMessage(e));
        }
    };

    const handleCreateAddon = async () => {
        if (!managingAddonsPlan?.id) return;
        try {
            await bizBillingAPI.addPlanAddon(apiKey, managingAddonsPlan.id, {
                name: addonName,
                amount_paisa: Math.round(addonPrice * 100),
                billing_cycle: addonCycle,
            });
            toast.success('Addon created');
            setAddonName('');
            setAddonPrice(0);
            // reload addons
            const data = await bizBillingAPI.listPlanAddons(apiKey, managingAddonsPlan.id);
            setAddons(data || []);
        } catch (e) {
            toast.error('Failed to create addon: ' + extractErrorMessage(e));
        }
    };

    const handleDeleteAddon = async (addonId: string) => {
        if (!managingAddonsPlan?.id) return;
        try {
            await bizBillingAPI.removePlanAddon(apiKey, managingAddonsPlan.id, addonId);
            toast.success('Addon removed');
            // reload addons
            const data = await bizBillingAPI.listPlanAddons(apiKey, managingAddonsPlan.id);
            setAddons(data || []);
        } catch (e) {
            toast.error('Failed to remove addon: ' + extractErrorMessage(e));
        }
    };

    // ── Subscription Actions ──

    const handleCreateSubscription = async () => {
        try {
            await bizBillingAPI.createSubscription(apiKey, {
                user_id: newSubUserId,
                plan_id: newSubPlanId,
                quantity: newSubQty,
                trial_days: newSubTrial,
            });
            toast.success('Subscription created');
            setIsCreateSubOpen(false);
            setNewSubUserId('');
            void load();
        } catch (e) {
            toast.error('Failed to create subscription: ' + extractErrorMessage(e));
        }
    };

    const handlePauseSubscription = async (id: string) => {
        setActingSubId(id);
        try {
            await bizBillingAPI.pauseSubscription(apiKey, id);
            toast.success('Subscription paused');
            void load();
        } catch (e) {
            toast.error('Failed to pause: ' + extractErrorMessage(e));
        } finally {
            setActingSubId(null);
        }
    };

    const handleResumeSubscription = async (id: string) => {
        setActingSubId(id);
        try {
            await bizBillingAPI.resumeSubscription(apiKey, id);
            toast.success('Subscription resumed');
            void load();
        } catch (e) {
            toast.error('Failed to resume: ' + extractErrorMessage(e));
        } finally {
            setActingSubId(null);
        }
    };

    const handleCancelSubscription = async (id: string) => {
        setActingSubId(id);
        try {
            await bizBillingAPI.cancelSubscription(apiKey, id, false);
            toast.success('Subscription canceled');
            void load();
        } catch (e) {
            toast.error('Failed to cancel: ' + extractErrorMessage(e));
        } finally {
            setActingSubId(null);
        }
    };

    // ── Invoice Actions ──

    const handleCreateInvoice = async () => {
        try {
            await bizBillingAPI.createInvoice(apiKey, {
                user_id: newInvUserId,
                line_items: [{
                    description: newInvDesc,
                    quantity: newInvQty,
                    unit_price: Math.round(newInvUnitPrice * 100),
                    tax_rate: newInvTaxRate,
                }],
                currency: newInvCurrency,
                payment_terms: newInvTerms,
            });
            toast.success('Invoice created');
            setIsCreateInvOpen(false);
            setNewInvUserId('');
            void load();
        } catch (e) {
            toast.error('Failed to create invoice: ' + extractErrorMessage(e));
        }
    };

    const handleIssueInvoice = async (id: string) => {
        setActingInvoiceId(id);
        try {
            await bizBillingAPI.issueInvoice(apiKey, id);
            toast.success('Invoice issued');
            void load();
        } catch (e) {
            toast.error('Failed to issue: ' + extractErrorMessage(e));
        } finally {
            setActingInvoiceId(null);
        }
    };

    const handleSendInvoice = async (id: string) => {
        setActingInvoiceId(id);
        try {
            await bizBillingAPI.sendInvoice(apiKey, id);
            toast.success('Invoice sent');
            void load();
        } catch (e) {
            toast.error('Failed to send: ' + extractErrorMessage(e));
        } finally {
            setActingInvoiceId(null);
        }
    };

    const handleRefundPayment = async (id: string) => {
        try {
            await bizBillingAPI.refundPayment(apiKey, id, {});
            toast.success('Payment refunded');
            void load();
        } catch (e) {
            toast.error('Failed to refund: ' + extractErrorMessage(e));
        }
    };

    const handleVoidInvoice = async (id: string) => {
        setActingInvoiceId(id);
        try {
            await bizBillingAPI.voidInvoice(apiKey, id);
            toast.success('Invoice voided');
            void load();
        } catch (e) {
            toast.error('Failed to void: ' + extractErrorMessage(e));
        } finally {
            setActingInvoiceId(null);
        }
    };

    const handleWriteOffInvoice = async (id: string) => {
        setActingInvoiceId(id);
        try {
            await bizBillingAPI.writeOffInvoice(apiKey, id);
            toast.success('Invoice written off');
            void load();
        } catch (e) {
            toast.error('Failed to write off: ' + extractErrorMessage(e));
        } finally {
            setActingInvoiceId(null);
        }
    };

    const statusBadge = (status: string) => {
        const variants: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
            active: 'default',
            trialing: 'default',
            paused: 'secondary',
            canceled: 'destructive',
            past_due: 'destructive',
            unpaid: 'destructive',
            draft: 'secondary',
            open: 'default',
            paid: 'default',
            void: 'destructive',
            uncollectible: 'destructive',
            overdue: 'destructive',
            success: 'default',
            failed: 'destructive',
            pending: 'secondary',
            refunded: 'outline',
        };
        return <Badge variant={variants[status] || 'secondary'}>{status}</Badge>;
    };

    const paisa = (n: number = 0) => (n / 100).toFixed(2);

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-2xl font-bold tracking-tight">Billing Setup</h2>
                    <p className="text-muted-foreground mt-1">
                        Manage the products and plans you offer to your customers.
                    </p>
                </div>
                <Button variant="outline" size="sm" onClick={load} disabled={loading}>
                    <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
                    Refresh
                </Button>
            </div>

            <Tabs defaultValue="products">
                <TabsList className="flex flex-wrap h-auto gap-1">
                    <TabsTrigger value="products">Products</TabsTrigger>
                    <TabsTrigger value="plans">Plans</TabsTrigger>
                    <TabsTrigger value="subscriptions">Subscriptions</TabsTrigger>
                    <TabsTrigger value="invoices">Invoices</TabsTrigger>
                    <TabsTrigger value="payments">Payments</TabsTrigger>
                    <TabsTrigger value="sales">Sales</TabsTrigger>
                    <TabsTrigger value="ops">Ops</TabsTrigger>
                    <TabsTrigger value="insights">Insights</TabsTrigger>
                    <TabsTrigger value="config">Config</TabsTrigger>
                </TabsList>

                <TabsContent value="products" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader className="flex flex-row items-center justify-between">
                            <div>
                                <CardTitle>Products</CardTitle>
                                <CardDescription>High-level offerings (e.g., API Access, Pro Dashboard).</CardDescription>
                            </div>
                            <Dialog open={isProductOpen} onOpenChange={setIsProductOpen}>
                                <DialogTrigger asChild>
                                    <Button size="sm"><Plus className="w-4 h-4 mr-1" /> New Product</Button>
                                </DialogTrigger>
                                <DialogContent>
                                    <DialogHeader>
                                        <DialogTitle>Create Product</DialogTitle>
                                        <DialogDescription className="sr-only">Form to create a new product.</DialogDescription>
                                    </DialogHeader>
                                    <div className="space-y-4 py-4">
                                        <div className="space-y-2">
                                            <Label>Name</Label>
                                            <Input value={prodName} onChange={e => setProdName(e.target.value)} placeholder="Pro Tier" />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Description</Label>
                                            <Input value={prodDesc} onChange={e => setProdDesc(e.target.value)} placeholder="Access to all pro features" />
                                        </div>
                                    </div>
                                    <DialogFooter>
                                        <Button onClick={handleCreateProduct}>Create</Button>
                                    </DialogFooter>
                                </DialogContent>
                            </Dialog>
                        </CardHeader>
                        <CardContent>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Name</TableHead>
                                        <TableHead>Description</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead className="text-right">Actions</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {products.map(p => (
                                        <TableRow key={p.id}>
                                            <TableCell className="font-medium">{p.name}</TableCell>
                                            <TableCell>{p.description}</TableCell>
                                            <TableCell><Badge variant={p.active ? 'default' : 'secondary'}>{p.active ? 'Active' : 'Inactive'}</Badge></TableCell>
                                            <TableCell className="text-right space-x-1">
                                                <Button variant="ghost" size="sm" onClick={() => handleEditProduct(p)}><Edit className="w-4 h-4" /></Button>
                                                <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={(e) => { e.stopPropagation(); p.id && setDeletingProductId(p.id); }}><Trash2 className="w-4 h-4" /></Button>
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                    {products.length === 0 && (
                                        <TableRow>
                                            <TableCell colSpan={4} className="text-center py-6 text-muted-foreground">No products found.</TableCell>
                                        </TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>

                    {/* Edit Product Dialog */}
                    <Dialog open={isEditProductOpen} onOpenChange={setIsEditProductOpen}>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Edit Product</DialogTitle>
                                <DialogDescription className="sr-only">Form to edit an existing product.</DialogDescription>
                            </DialogHeader>
                            <div className="space-y-4 py-4">
                                <div className="space-y-2">
                                    <Label>Name</Label>
                                    <Input value={editProdName} onChange={e => setEditProdName(e.target.value)} />
                                </div>
                                <div className="space-y-2">
                                    <Label>Description</Label>
                                    <Input value={editProdDesc} onChange={e => setEditProdDesc(e.target.value)} />
                                </div>
                                <div className="flex items-center space-x-2">
                                    <input type="checkbox" id="editProdActive" checked={editProdActive} onChange={e => setEditProdActive(e.target.checked)} className="h-4 w-4 rounded border-gray-300" />
                                    <Label htmlFor="editProdActive">Active</Label>
                                </div>
                            </div>
                            <DialogFooter>
                                <Button variant="outline" onClick={() => setIsEditProductOpen(false)}>Cancel</Button>
                                <Button onClick={handleUpdateProduct}>Save Changes</Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                    {/* Delete Product Confirmation Dialog */}
                    <Dialog open={!!deletingProductId} onOpenChange={(open) => { if (!open) setDeletingProductId(null); }}>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Archive Product</DialogTitle>
                                <DialogDescription>Are you sure you want to archive this product? This action can be undone later.</DialogDescription>
                            </DialogHeader>
                            <DialogFooter>
                                <Button variant="outline" onClick={() => setDeletingProductId(null)}>Cancel</Button>
                                <Button variant="destructive" onClick={handleDeleteProduct}>Archive</Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </TabsContent>

                <TabsContent value="plans" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader className="flex flex-row items-center justify-between">
                            <div>
                                <CardTitle>Plans</CardTitle>
                                <CardDescription>Pricing tiers attached to products.</CardDescription>
                            </div>
                            <Dialog open={isPlanOpen} onOpenChange={setIsPlanOpen}>
                                <Button size="sm" onClick={() => {
                                    if (products.length > 0) {
                                        setPlanProduct(products[0].id || '');
                                    } else {
                                        setPlanProduct('');
                                    }
                                    setIsPlanOpen(true);
                                }}><Plus className="w-4 h-4 mr-1" /> New Plan</Button>
                                <DialogContent>
                                    <DialogHeader>
                                        <DialogTitle>Create Plan</DialogTitle>
                                        <DialogDescription className="sr-only">Form to create a new plan.</DialogDescription>
                                    </DialogHeader>
                                    <div className="space-y-4 py-4">
                                        <div className="space-y-2">
                                            <Label>Product</Label>
                                            <select
                                                value={planProduct}
                                                onChange={e => setPlanProduct(e.target.value)}
                                                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                            >
                                                <option value="">Select a product</option>
                                                {products.map(p => (
                                                    <option key={p.id} value={p.id}>{p.name} ({p.id})</option>
                                                ))}
                                            </select>
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Name</Label>
                                            <Input value={planName} onChange={e => setPlanName(e.target.value)} placeholder="Monthly Pro" />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Description</Label>
                                            <Input value={planDesc} onChange={e => setPlanDesc(e.target.value)} placeholder="Billed monthly" />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Price (USD)</Label>
                                            <Input type="number" value={planPrice} onChange={e => setPlanPrice(Number(e.target.value))} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Billing Cycle</Label>
                                            <select
                                                value={planCycle}
                                                onChange={e => setPlanCycle(e.target.value)}
                                                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                            >
                                                <option value="monthly">Monthly</option>
                                                <option value="yearly">Yearly</option>
                                                <option value="one-time">One-Time</option>
                                            </select>
                                        </div>
                                    </div>
                                    <DialogFooter>
                                        <Button onClick={handleCreatePlan}>Create</Button>
                                    </DialogFooter>
                                </DialogContent>
                            </Dialog>
                        </CardHeader>
                        <CardContent>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Name</TableHead>
                                        <TableHead>Cycle</TableHead>
                                        <TableHead>Price</TableHead>
                                        <TableHead>Status</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {plans.map(p => (
                                        <TableRow key={p.id}>
                                            <TableCell className="font-medium">{p.name}</TableCell>
                                            <TableCell className="capitalize">{p.billing_cycle || 'N/A'}</TableCell>
                                            <TableCell>{(p.amount_paisa || 0) / 100} USD</TableCell>
                                            <TableCell><Badge variant={p.active ? 'default' : 'secondary'}>{p.active ? 'Active' : 'Inactive'}</Badge></TableCell>
                                            <TableCell className="text-right space-x-1">
                                                <Button variant="ghost" size="sm" onClick={() => handleManageAddons(p)}>Addons</Button>
                                                <Button variant="ghost" size="sm" onClick={() => handleEditPlan(p)}><Edit className="w-4 h-4" /></Button>
                                                <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={(e) => { e.stopPropagation(); p.id && setDeletingPlanId(p.id); }}><Trash2 className="w-4 h-4" /></Button>
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                    {plans.length === 0 && (
                                        <TableRow>
                                            <TableCell colSpan={4} className="text-center py-6 text-muted-foreground">No plans found.</TableCell>
                                        </TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>

                    {/* Edit Plan Dialog */}
                    <Dialog open={isEditPlanOpen} onOpenChange={setIsEditPlanOpen}>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Edit Plan</DialogTitle>
                                <DialogDescription className="sr-only">Form to edit an existing plan.</DialogDescription>
                            </DialogHeader>
                            <div className="space-y-4 py-4">
                                <div className="space-y-2">
                                    <Label>Name</Label>
                                    <Input value={editPlanName} onChange={e => setEditPlanName(e.target.value)} />
                                </div>
                                <div className="space-y-2">
                                    <Label>Description</Label>
                                    <Input value={editPlanDesc} onChange={e => setEditPlanDesc(e.target.value)} />
                                </div>
                                <div className="space-y-2">
                                    <Label>Price (USD)</Label>
                                    <Input type="number" value={editPlanPrice} onChange={e => setEditPlanPrice(Number(e.target.value))} />
                                </div>
                                <div className="space-y-2">
                                    <Label>Billing Cycle</Label>
                                    <select
                                        value={editPlanCycle}
                                        onChange={e => setEditPlanCycle(e.target.value)}
                                        className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                    >
                                        <option value="monthly">Monthly</option>
                                        <option value="yearly">Yearly</option>
                                        <option value="one-time">One-Time</option>
                                    </select>
                                </div>
                            </div>
                            <DialogFooter>
                                <Button variant="outline" onClick={() => setIsEditPlanOpen(false)}>Cancel</Button>
                                <Button onClick={handleUpdatePlan}>Save Changes</Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>

                    {/* Delete Plan Confirmation Dialog */}
                    <Dialog open={!!deletingPlanId} onOpenChange={(open) => { if (!open) setDeletingPlanId(null); }}>
                        <DialogContent>
                            <DialogHeader>
                                <DialogTitle>Archive Plan</DialogTitle>
                                <DialogDescription>Are you sure you want to archive this plan? This action can be undone later.</DialogDescription>
                            </DialogHeader>
                            <DialogFooter>
                                <Button variant="outline" onClick={() => setDeletingPlanId(null)}>Cancel</Button>
                                <Button variant="destructive" onClick={handleDeletePlan}>Archive</Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>

                    {/* Manage Addons Dialog */}
                    <Dialog open={isAddonsOpen} onOpenChange={setIsAddonsOpen}>
                        <DialogContent className="max-w-2xl">
                            <DialogHeader>
                                <DialogTitle>Manage Addons - {managingAddonsPlan?.name}</DialogTitle>
                                <DialogDescription>Create and manage addons available for this plan.</DialogDescription>
                            </DialogHeader>
                            <div className="space-y-4 py-4">
                                <div className="flex space-x-2">
                                    <Input value={addonName} onChange={e => setAddonName(e.target.value)} placeholder="Addon Name" className="flex-1" />
                                    <Input type="number" value={addonPrice} onChange={e => setAddonPrice(Number(e.target.value))} placeholder="Price (USD)" className="w-32" />
                                    <select
                                        value={addonCycle}
                                        onChange={e => setAddonCycle(e.target.value)}
                                        className="flex h-9 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                                    >
                                        <option value="monthly">Monthly</option>
                                        <option value="yearly">Yearly</option>
                                        <option value="one-time">One-Time</option>
                                    </select>
                                    <Button onClick={handleCreateAddon}>Add</Button>
                                </div>
                                <div className="rounded-md border mt-4 overflow-hidden">
                                    <Table>
                                        <TableHeader>
                                            <TableRow>
                                                <TableHead>Name</TableHead>
                                                <TableHead>Price</TableHead>
                                                <TableHead>Cycle</TableHead>
                                                <TableHead className="w-16"></TableHead>
                                            </TableRow>
                                        </TableHeader>
                                        <TableBody>
                                            {addons.map(a => (
                                                <TableRow key={a.id}>
                                                    <TableCell>{a.name}</TableCell>
                                                    <TableCell>{(a.amount_paisa || 0) / 100} USD</TableCell>
                                                    <TableCell className="capitalize">{a.billing_cycle || 'N/A'}</TableCell>
                                                    <TableCell>
                                                        <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" onClick={() => a.id && handleDeleteAddon(a.id)}>
                                                            <Trash2 className="w-4 h-4" />
                                                        </Button>
                                                    </TableCell>
                                                </TableRow>
                                            ))}
                                            {addons.length === 0 && (
                                                <TableRow>
                                                    <TableCell colSpan={4} className="text-center py-4 text-muted-foreground">No addons for this plan.</TableCell>
                                                </TableRow>
                                            )}
                                        </TableBody>
                                    </Table>
                                </div>
                            </div>
                        </DialogContent>
                    </Dialog>
                </TabsContent>

                <TabsContent value="subscriptions" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader className="flex flex-row items-center justify-between">
                            <div>
                                <CardTitle>Subscriptions</CardTitle>
                                <CardDescription>Manage customer subscriptions.</CardDescription>
                            </div>
                            <Dialog open={isCreateSubOpen} onOpenChange={setIsCreateSubOpen}>
                                <DialogTrigger asChild>
                                    <Button size="sm"><Plus className="w-4 h-4 mr-1" /> Create Subscription</Button>
                                </DialogTrigger>
                                <DialogContent>
                                    <DialogHeader>
                                        <DialogTitle>Create Subscription</DialogTitle>
                                        <DialogDescription className="sr-only">Form to create a new subscription.</DialogDescription>
                                    </DialogHeader>
                                    <div className="space-y-4 py-4">
                                        <div className="space-y-2">
                                            <Label>User ID</Label>
                                            <Input value={newSubUserId} onChange={e => setNewSubUserId(e.target.value)} placeholder="user_xxx" />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Plan</Label>
                                            <select
                                                value={newSubPlanId}
                                                onChange={e => setNewSubPlanId(e.target.value)}
                                                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                            >
                                                <option value="">Select a plan</option>
                                                {plans.filter(p => p.active).map(p => (
                                                    <option key={p.id} value={p.id}>{p.name} — ${paisa(p.amount_paisa || 0)}/{p.billing_cycle}</option>
                                                ))}
                                            </select>
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Quantity</Label>
                                            <Input type="number" min={1} value={newSubQty} onChange={e => setNewSubQty(Number(e.target.value))} />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Trial Days</Label>
                                            <Input type="number" min={0} value={newSubTrial} onChange={e => setNewSubTrial(Number(e.target.value))} placeholder="0 = no trial" />
                                        </div>
                                    </div>
                                    <DialogFooter>
                                        <Button onClick={handleCreateSubscription}>Create</Button>
                                    </DialogFooter>
                                </DialogContent>
                            </Dialog>
                        </CardHeader>
                        <CardContent>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>User ID</TableHead>
                                        <TableHead>Plan</TableHead>
                                        <TableHead>Amount</TableHead>
                                        <TableHead>Cycle</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead>Period</TableHead>
                                        <TableHead className="text-right">Actions</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {subscriptions.map(s => (
                                        <TableRow key={s.id}>
                                            <TableCell className="font-medium text-xs">{s.user_id}</TableCell>
                                            <TableCell>{s.plan_name || s.plan_id}</TableCell>
                                            <TableCell>${paisa(s.plan_amount_paisa)}</TableCell>
                                            <TableCell className="capitalize">{s.billing_cycle || '–'}</TableCell>
                                            <TableCell>{statusBadge(s.status)}</TableCell>
                                            <TableCell className="text-xs text-muted-foreground">
                                                {s.current_period_start ? new Date(s.current_period_start).toLocaleDateString() : '–'}
                                                {' → '}
                                                {s.current_period_end ? new Date(s.current_period_end).toLocaleDateString() : '–'}
                                            </TableCell>
                                            <TableCell className="text-right space-x-1">
                                                {s.status === 'active' && (
                                                    <>
                                                        <Button variant="ghost" size="sm" disabled={actingSubId === s.id} onClick={() => handlePauseSubscription(s.id)} title="Pause">
                                                            <Pause className="w-4 h-4" />
                                                        </Button>
                                                        <Button variant="ghost" size="sm" disabled={actingSubId === s.id} onClick={() => handleCancelSubscription(s.id)} title="Cancel" className="text-destructive hover:text-destructive">
                                                            <XCircle className="w-4 h-4" />
                                                        </Button>
                                                    </>
                                                )}
                                                {s.status === 'paused' && (
                                                    <Button variant="ghost" size="sm" disabled={actingSubId === s.id} onClick={() => handleResumeSubscription(s.id)} title="Resume">
                                                        <Play className="w-4 h-4" />
                                                    </Button>
                                                )}
                                                {actingSubId === s.id && (
                                                    <Loader2 className="w-4 h-4 animate-spin inline-block" />
                                                )}
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                    {subscriptions.length === 0 && (
                                        <TableRow><TableCell colSpan={7} className="text-center py-6 text-muted-foreground">No subscriptions found.</TableCell></TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="invoices" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader className="flex flex-row items-center justify-between">
                            <div>
                                <CardTitle>Invoices</CardTitle>
                                <CardDescription>Manage billing invoices.</CardDescription>
                            </div>
                            <Dialog open={isCreateInvOpen} onOpenChange={setIsCreateInvOpen}>
                                <DialogTrigger asChild>
                                    <Button size="sm"><Plus className="w-4 h-4 mr-1" /> Create Invoice</Button>
                                </DialogTrigger>
                                <DialogContent>
                                    <DialogHeader>
                                        <DialogTitle>Create One-Time Invoice</DialogTitle>
                                        <DialogDescription className="sr-only">Form to create a new invoice.</DialogDescription>
                                    </DialogHeader>
                                    <div className="space-y-4 py-4">
                                        <div className="space-y-2">
                                            <Label>User ID</Label>
                                            <Input value={newInvUserId} onChange={e => setNewInvUserId(e.target.value)} placeholder="user_xxx" />
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Description</Label>
                                            <Input value={newInvDesc} onChange={e => setNewInvDesc(e.target.value)} placeholder="Consulting fee" />
                                        </div>
                                        <div className="grid grid-cols-2 gap-4">
                                            <div className="space-y-2">
                                                <Label>Quantity</Label>
                                                <Input type="number" min={1} value={newInvQty} onChange={e => setNewInvQty(Number(e.target.value))} />
                                            </div>
                                            <div className="space-y-2">
                                                <Label>Unit Price ({newInvCurrency})</Label>
                                                <Input type="number" min={0} step="0.01" value={newInvUnitPrice} onChange={e => setNewInvUnitPrice(Number(e.target.value))} />
                                            </div>
                                        </div>
                                        <div className="grid grid-cols-2 gap-4">
                                            <div className="space-y-2">
                                                <Label>Tax Rate (%)</Label>
                                                <Input type="number" min={0} max={100} value={newInvTaxRate} onChange={e => setNewInvTaxRate(Number(e.target.value))} />
                                            </div>
                                            <div className="space-y-2">
                                                <Label>Currency</Label>
                                                <select
                                                    value={newInvCurrency}
                                                    onChange={e => setNewInvCurrency(e.target.value)}
                                                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                                >
                                                    <option value="INR">INR</option>
                                                    <option value="USD">USD</option>
                                                    <option value="EUR">EUR</option>
                                                </select>
                                            </div>
                                        </div>
                                        <div className="space-y-2">
                                            <Label>Payment Terms</Label>
                                            <select
                                                value={newInvTerms}
                                                onChange={e => setNewInvTerms(e.target.value)}
                                                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                            >
                                                <option value="due_on_receipt">Due on Receipt</option>
                                                <option value="net15">Net 15</option>
                                                <option value="net30">Net 30</option>
                                                <option value="net60">Net 60</option>
                                            </select>
                                        </div>
                                    </div>
                                    <DialogFooter>
                                        <Button onClick={handleCreateInvoice}>Create Draft Invoice</Button>
                                    </DialogFooter>
                                </DialogContent>
                            </Dialog>
                        </CardHeader>
                        <CardContent>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Invoice #</TableHead>
                                        <TableHead>User ID</TableHead>
                                        <TableHead>Amount Due</TableHead>
                                        <TableHead>Amount Paid</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead>Issued At</TableHead>
                                        <TableHead className="text-right">Actions</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {invoices.map(i => (
                                        <TableRow key={i.id}>
                                            <TableCell className="font-medium">{i.invoice_number}</TableCell>
                                            <TableCell className="text-xs">{i.user_id}</TableCell>
                                            <TableCell>${paisa(i.amount_due_paisa)} {i.currency}</TableCell>
                                            <TableCell>${paisa(i.total_paisa)}</TableCell>
                                            <TableCell>{statusBadge(i.status)}</TableCell>
                                            <TableCell className="text-xs text-muted-foreground">
                                                {i.issued_at ? new Date(i.issued_at).toLocaleDateString() : '–'}
                                            </TableCell>
                                            <TableCell className="text-right space-x-1">
                                                {i.status === 'draft' && (
                                                    <>
                                                        <Button variant="ghost" size="sm" disabled={actingInvoiceId === i.id} onClick={() => handleIssueInvoice(i.id)} title="Issue">
                                                            <FileText className="w-4 h-4" />
                                                        </Button>
                                                        <Button variant="ghost" size="sm" disabled={actingInvoiceId === i.id} onClick={() => handleVoidInvoice(i.id)} title="Void" className="text-destructive hover:text-destructive">
                                                            <Ban className="w-4 h-4" />
                                                        </Button>
                                                    </>
                                                )}
                                                {i.status === 'open' && (
                                                    <>
                                                        <Button variant="ghost" size="sm" disabled={actingInvoiceId === i.id} onClick={() => handleSendInvoice(i.id)} title="Send">
                                                            Send
                                                        </Button>
                                                        <Button variant="ghost" size="sm" disabled={actingInvoiceId === i.id} onClick={() => handleVoidInvoice(i.id)} title="Void" className="text-destructive hover:text-destructive">
                                                            <Ban className="w-4 h-4" />
                                                        </Button>
                                                        <Button variant="ghost" size="sm" disabled={actingInvoiceId === i.id} onClick={() => handleWriteOffInvoice(i.id)} title="Write Off">
                                                            <XCircle className="w-4 h-4" />
                                                        </Button>
                                                    </>
                                                )}
                                                {actingInvoiceId === i.id && (
                                                    <Loader2 className="w-4 h-4 animate-spin inline-block" />
                                                )}
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                    {invoices.length === 0 && (
                                        <TableRow><TableCell colSpan={7} className="text-center py-6 text-muted-foreground">No invoices found.</TableCell></TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="payments" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader>
                            <CardTitle>Payments</CardTitle>
                            <CardDescription>Recorded payments and transactions.</CardDescription>
                        </CardHeader>
                        <CardContent>
                            <Table>
                                <TableHeader>
                                    <TableRow>
                                        <TableHead>Payment ID</TableHead>
                                        <TableHead>Amount</TableHead>
                                        <TableHead>Method</TableHead>
                                        <TableHead>Status</TableHead>
                                        <TableHead>Gateway Txn ID</TableHead>
                                        <TableHead>Date</TableHead>
                                        <TableHead className="text-right">Actions</TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    {payments.map(p => (
                                        <TableRow key={p.id}>
                                            <TableCell className="font-medium text-xs">{p.id}</TableCell>
                                            <TableCell>${paisa(p.amount_paisa)} {p.currency}</TableCell>
                                            <TableCell className="capitalize">{p.method || '–'}</TableCell>
                                            <TableCell>{statusBadge(p.status)}</TableCell>
                                            <TableCell className="text-xs">{p.gateway_payment_id || '–'}</TableCell>
                                            <TableCell className="text-xs text-muted-foreground">
                                                {p.created_at ? new Date(p.created_at).toLocaleDateString() : '–'}
                                            </TableCell>
                                            <TableCell className="text-right">
                                                {p.status === 'success' && (
                                                    <Button variant="outline" size="sm" onClick={() => handleRefundPayment(p.id)}>
                                                        Refund
                                                    </Button>
                                                )}
                                            </TableCell>
                                        </TableRow>
                                    ))}
                                    {payments.length === 0 && (
                                        <TableRow><TableCell colSpan={7} className="text-center py-6 text-muted-foreground">No payments found.</TableCell></TableRow>
                                    )}
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="sales" className="pt-4">
                    <BizBillingSalesPanel apiKey={apiKey} />
                </TabsContent>

                <TabsContent value="ops" className="pt-4">
                    <BizBillingOpsPanel apiKey={apiKey} />
                </TabsContent>

                <TabsContent value="insights" className="pt-4">
                    <BizBillingInsightsPanel apiKey={apiKey} />
                </TabsContent>

                <TabsContent value="config" className="space-y-4 pt-4">
                    <Card>
                        <CardHeader>
                            <CardTitle>Provider Configuration</CardTitle>
                            <CardDescription>JSON configuration for Stripe/Razorpay keys.</CardDescription>
                        </CardHeader>
                        <CardContent>
                            <Textarea
                                className="font-mono h-64 text-sm"
                                value={configText}
                                onChange={e => setConfigText(e.target.value)}
                            />
                            <Button className="mt-4" onClick={handleSaveConfig} disabled={loading}>
                                {loading ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : null}
                                Save Config
                            </Button>
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div >
    );
};

export default AppBizBilling;
