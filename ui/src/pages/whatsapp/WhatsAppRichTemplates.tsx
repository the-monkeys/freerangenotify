import React, { useState } from 'react';
import { whatsappRichTemplatesAPI } from '../../services/api';
import { useApiQuery } from '../../hooks/use-api-query';
import { Card, CardHeader, CardTitle, CardContent } from '../../components/ui/card';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Badge } from '../../components/ui/badge';
import { Spinner } from '../../components/ui/spinner';
import EmptyState from '../../components/EmptyState';
import { toast } from 'sonner';
import { Plus, RefreshCw, Trash2, Eye, Layers, Tag, ExternalLink } from 'lucide-react';

interface Props {
    apiKey: string;
    appId: string;
}

// approvalColors maps the aggregate ApprovalState (set server-side from
// per-provider statuses) to a Tailwind badge class. Keep this in sync with
// internal/domain/whatsapp/rich.go ApprovalState constants.
const approvalColors: Record<string, string> = {
    draft: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
    pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
    partially_submitted: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200',
    approved: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
    rejected: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
    disabled: 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200',
};

// Shape of a card slot in the carousel builder form. Mirrors
// internal/domain/whatsapp/rich.go CarouselCard so the POST body needs no
// transformation beyond JSON serialisation.
type DraftCard = {
    header_image_url: string;
    body: string;
    button_text: string;
    button_url: string;
};

type DraftKind = 'carousel' | 'coupon_code';

const emptyCard = (): DraftCard => ({ header_image_url: '', body: '', button_text: 'View', button_url: '' });

const WhatsAppRichTemplates: React.FC<Props> = ({ apiKey, appId }) => {
    const [showCreate, setShowCreate] = useState(false);
    const [creating, setCreating] = useState(false);
    const [kind, setKind] = useState<DraftKind>('carousel');
    const [draft, setDraft] = useState({
        name: '',
        category: 'MARKETING',
        language: 'en_US',
        body: '',
        coupon_code: '',
        cards: [emptyCard(), emptyCard()] as DraftCard[],
    });

    const { data, loading, refetch } = useApiQuery(
        () => whatsappRichTemplatesAPI.list(apiKey),
        [apiKey],
        { enabled: !!apiKey, cacheKey: `wa-rich-templates-${appId}` },
    );
    const templates: any[] = data?.templates || [];

    const addCard = () => setDraft({ ...draft, cards: [...draft.cards, emptyCard()] });
    const removeCard = (idx: number) =>
        setDraft({ ...draft, cards: draft.cards.filter((_, i) => i !== idx) });
    const updateCard = (idx: number, patch: Partial<DraftCard>) =>
        setDraft({ ...draft, cards: draft.cards.map((c, i) => (i === idx ? { ...c, ...patch } : c)) });

    const handleCreate = async () => {
        if (!draft.name) {
            toast.error('Template name is required');
            return;
        }
        const payload: any = {
            name: draft.name,
            kind,
            category: draft.category,
            language: draft.language,
            body: draft.body,
        };
        if (kind === 'carousel') {
            if (draft.cards.length < 2 || draft.cards.length > 10) {
                toast.error('Carousel requires 2 to 10 cards');
                return;
            }
            payload.cards = draft.cards.map((c) => ({
                header_image_url: c.header_image_url,
                body: c.body,
                buttons: c.button_url
                    ? [{ type: 'URL', text: c.button_text, url: c.button_url, track_clicks: true }]
                    : [],
            }));
        } else if (kind === 'coupon_code') {
            if (!draft.coupon_code) {
                toast.error('Coupon code is required');
                return;
            }
            payload.coupon_code = draft.coupon_code;
        }

        setCreating(true);
        try {
            await whatsappRichTemplatesAPI.create(apiKey, payload);
            toast.success('Rich template submitted');
            setShowCreate(false);
            setDraft({
                name: '',
                category: 'MARKETING',
                language: 'en_US',
                body: '',
                coupon_code: '',
                cards: [emptyCard(), emptyCard()],
            });
            refetch();
        } catch (err: any) {
            // Validation errors come back as { error, details: [{field, message, code}] }
            const details = err.response?.data?.details;
            if (Array.isArray(details) && details.length > 0) {
                toast.error(`Validation: ${details.map((d: any) => `${d.field}: ${d.message}`).join('; ')}`);
            } else {
                toast.error('Create failed: ' + (err.response?.data?.error || err.message));
            }
        } finally {
            setCreating(false);
        }
    };

    const handleDelete = async (id: string, name: string) => {
        if (!confirm(`Delete rich template "${name}"? This cannot be undone.`)) return;
        try {
            await whatsappRichTemplatesAPI.delete(apiKey, id);
            toast.success('Template deleted');
            refetch();
        } catch (err: any) {
            toast.error('Delete failed: ' + (err.response?.data?.error || err.message));
        }
    };

    const handleSync = async (id: string) => {
        try {
            await whatsappRichTemplatesAPI.sync(apiKey, id);
            toast.success('Synced from Meta');
            refetch();
        } catch (err: any) {
            toast.error('Sync failed: ' + (err.response?.data?.error || err.message));
        }
    };

    const handlePreview = async (id: string) => {
        try {
            const preview = await whatsappRichTemplatesAPI.preview(apiKey, id);
            // Open a side window with the JSON; cheap for now, the UI side-
            // by-side preview is a follow-up.
            const w = window.open('', '_blank', 'width=600,height=800');
            if (w) {
                w.document.write(
                    `<pre style="padding:1rem;font:13px/1.4 monospace">${escapeHTML(
                        JSON.stringify(preview, null, 2),
                    )}</pre>`,
                );
            }
        } catch (err: any) {
            toast.error('Preview failed: ' + (err.response?.data?.error || err.message));
        }
    };

    if (loading) {
        return (
            <div className="flex items-center justify-center py-16">
                <Spinner className="h-6 w-6" />
            </div>
        );
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-lg font-semibold">WhatsApp Rich Templates</h3>
                    <p className="text-sm text-muted-foreground">
                        Carousel and coupon-code templates with click attribution. Submitted to Meta on save; approval typically lands in &lt; 1 minute.
                    </p>
                </div>
                <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => refetch()}>
                        <RefreshCw className="h-4 w-4 mr-1" /> Refresh
                    </Button>
                    <Button size="sm" onClick={() => setShowCreate(!showCreate)}>
                        <Plus className="h-4 w-4 mr-1" /> New Rich Template
                    </Button>
                </div>
            </div>

            {showCreate && (
                <Card>
                    <CardHeader>
                        <CardTitle className="text-base">New Rich Template</CardTitle>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                            <div className="space-y-2">
                                <Label>Kind</Label>
                                <select
                                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                                    value={kind}
                                    onChange={(e) => setKind(e.target.value as DraftKind)}
                                >
                                    <option value="carousel">Carousel</option>
                                    <option value="coupon_code">Coupon Code</option>
                                </select>
                            </div>
                            <div className="space-y-2">
                                <Label>Name</Label>
                                <Input
                                    value={draft.name}
                                    onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                                    placeholder="diwali_carousel"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>Category</Label>
                                <select
                                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm"
                                    value={draft.category}
                                    onChange={(e) => setDraft({ ...draft, category: e.target.value })}
                                >
                                    <option value="MARKETING">Marketing</option>
                                    <option value="UTILITY">Utility</option>
                                    <option value="AUTHENTICATION">Authentication</option>
                                </select>
                            </div>
                            <div className="space-y-2">
                                <Label>Language</Label>
                                <Input
                                    value={draft.language}
                                    onChange={(e) => setDraft({ ...draft, language: e.target.value })}
                                    placeholder="en_US"
                                />
                            </div>
                        </div>

                        <div className="space-y-2">
                            <Label>Body</Label>
                            <Input
                                value={draft.body}
                                onChange={(e) => setDraft({ ...draft, body: e.target.value })}
                                placeholder="Hi {{1}}, check these out:"
                            />
                            <p className="text-xs text-muted-foreground">
                                Use {'{{1}}'}, {'{{2}}'} … for variables. Variables must start at 1 and be contiguous.
                            </p>
                        </div>

                        {kind === 'coupon_code' && (
                            <div className="space-y-2">
                                <Label>Coupon Code</Label>
                                <Input
                                    value={draft.coupon_code}
                                    onChange={(e) => setDraft({ ...draft, coupon_code: e.target.value })}
                                    placeholder="DEAL50"
                                />
                            </div>
                        )}

                        {kind === 'carousel' && (
                            <div className="space-y-3">
                                <div className="flex items-center justify-between">
                                    <Label>Cards ({draft.cards.length} of 10)</Label>
                                    <Button size="sm" variant="outline" onClick={addCard} disabled={draft.cards.length >= 10}>
                                        <Plus className="h-3.5 w-3.5 mr-1" /> Add Card
                                    </Button>
                                </div>
                                {draft.cards.map((c, i) => (
                                    <Card key={i} className="p-3">
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                            <div className="space-y-1">
                                                <Label className="text-xs">Header Image URL</Label>
                                                <Input
                                                    value={c.header_image_url}
                                                    onChange={(e) => updateCard(i, { header_image_url: e.target.value })}
                                                    placeholder="https://cdn.example.com/p1.jpg"
                                                />
                                            </div>
                                            <div className="space-y-1">
                                                <Label className="text-xs">Body</Label>
                                                <Input
                                                    value={c.body}
                                                    onChange={(e) => updateCard(i, { body: e.target.value })}
                                                    placeholder="Polo {{1}} {{2}}"
                                                />
                                            </div>
                                            <div className="space-y-1">
                                                <Label className="text-xs">Button Text</Label>
                                                <Input
                                                    value={c.button_text}
                                                    onChange={(e) => updateCard(i, { button_text: e.target.value })}
                                                />
                                            </div>
                                            <div className="space-y-1">
                                                <Label className="text-xs">Button URL (tracked)</Label>
                                                <Input
                                                    value={c.button_url}
                                                    onChange={(e) => updateCard(i, { button_url: e.target.value })}
                                                    placeholder="https://shop.example/p/{{1}}"
                                                />
                                            </div>
                                        </div>
                                        {draft.cards.length > 2 && (
                                            <div className="flex justify-end mt-2">
                                                <Button size="sm" variant="ghost" onClick={() => removeCard(i)}>
                                                    <Trash2 className="h-3.5 w-3.5 mr-1" /> Remove
                                                </Button>
                                            </div>
                                        )}
                                    </Card>
                                ))}
                            </div>
                        )}

                        <div className="flex justify-end gap-2 pt-2">
                            <Button variant="outline" onClick={() => setShowCreate(false)}>
                                Cancel
                            </Button>
                            <Button onClick={handleCreate} disabled={creating}>
                                {creating ? <Spinner className="h-4 w-4 mr-1" /> : null}
                                Submit to Meta
                            </Button>
                        </div>
                    </CardContent>
                </Card>
            )}

            {templates.length === 0 ? (
                <EmptyState
                    icon={<Layers className="h-12 w-12" />}
                    title="No rich templates yet"
                    description="Create a carousel or coupon-code template to ship visually richer WhatsApp messages."
                />
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                    {templates.map((t) => {
                        const meta = t.providers?.meta;
                        const twilio = t.providers?.twilio;
                        return (
                            <Card key={t.id} className="flex flex-col">
                                <CardHeader className="pb-2">
                                    <CardTitle className="text-base flex items-center justify-between gap-2">
                                        <span className="truncate">{t.name}</span>
                                        <Badge className={approvalColors[t.approval_state] || ''}>
                                            {t.approval_state}
                                        </Badge>
                                    </CardTitle>
                                    <p className="text-xs text-muted-foreground capitalize">
                                        {t.kind?.replace('_', ' ')} · {t.language} · {t.category}
                                    </p>
                                </CardHeader>
                                <CardContent className="flex-1 space-y-2 text-sm">
                                    {t.body && (
                                        <p className="text-muted-foreground line-clamp-2">{t.body}</p>
                                    )}
                                    {t.kind === 'carousel' && (
                                        <p className="text-xs text-muted-foreground">
                                            <Layers className="inline h-3 w-3 mr-1" />
                                            {t.cards?.length || 0} cards
                                        </p>
                                    )}
                                    {t.kind === 'coupon_code' && t.coupon_code && (
                                        <p className="text-xs text-muted-foreground">
                                            <Tag className="inline h-3 w-3 mr-1" />
                                            {t.coupon_code}
                                        </p>
                                    )}
                                    <div className="text-xs space-y-0.5">
                                        {meta?.template_id && (
                                            <p className="text-muted-foreground">
                                                <span className="font-medium">Meta:</span> {meta.status || '—'}
                                            </p>
                                        )}
                                        {twilio?.content_sid && (
                                            <p className="text-muted-foreground">
                                                <span className="font-medium">Twilio:</span> {twilio.status || '—'}
                                            </p>
                                        )}
                                    </div>
                                    <div className="flex flex-wrap gap-1 pt-2">
                                        <Button size="sm" variant="outline" onClick={() => handlePreview(t.id)}>
                                            <Eye className="h-3.5 w-3.5 mr-1" /> Preview
                                        </Button>
                                        <Button size="sm" variant="outline" onClick={() => handleSync(t.id)}>
                                            <RefreshCw className="h-3.5 w-3.5 mr-1" /> Sync
                                        </Button>
                                        <Button size="sm" variant="ghost" onClick={() => handleDelete(t.id, t.name)}>
                                            <Trash2 className="h-3.5 w-3.5" />
                                        </Button>
                                    </div>
                                </CardContent>
                            </Card>
                        );
                    })}
                </div>
            )}
        </div>
    );
};

// escapeHTML guards the JSON pre-window against XSS via header values
// reflected in preview output (paranoid; we control all sources today).
function escapeHTML(s: string): string {
    return s
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

// ExternalLink imported but only used implicitly via the design system;
// removing keeps lint clean. Re-export so future "Open in Meta Manager"
// affordances can pick it up without re-importing.
export { ExternalLink };

export default WhatsAppRichTemplates;
