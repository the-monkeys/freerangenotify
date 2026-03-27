import { useEffect, useMemo, useState, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { applicationsAPI, templatesAPI } from '../services/api';
import type { Template } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Skeleton } from '../components/ui/skeleton';
import { SlidePanel } from '../components/ui/slide-panel';
import { toast } from 'sonner';
import { ArrowLeft, Camera, Download, Expand, Loader2, Mail, Mic, MoreVertical, Paperclip, Phone, Smile, Video, Bell, MessageSquare, Globe, Radio, Webhook } from 'lucide-react';

const channelMeta: Record<string, { icon: React.ReactNode; label: string }> = {
    email: { icon: <Mail className="h-4 w-4" />, label: 'Email' },
    push: { icon: <Bell className="h-4 w-4" />, label: 'Push' },
    sms: { icon: <MessageSquare className="h-4 w-4" />, label: 'SMS' },
    webhook: { icon: <Webhook className="h-4 w-4" />, label: 'Webhook' },
    in_app: { icon: <Globe className="h-4 w-4" />, label: 'In-App' },
    sse: { icon: <Radio className="h-4 w-4" />, label: 'SSE' },
};

const categoryColors: Record<string, string> = {
    transactional: 'bg-sky-500/10 text-sky-700 border-sky-500/20',
    newsletter: 'bg-emerald-500/10 text-emerald-700 border-emerald-500/20',
    notification: 'bg-amber-500/10 text-amber-700 border-amber-500/20',
};

const formatCategory = (category: string) =>
    category.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());

function extractWhatsAppBody(rendered: string): string {
    if (!rendered) return '';

    const bodyMatch = rendered.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
    const fromBody = bodyMatch?.[1] ?? rendered;

    return fromBody
        .replace(/<script[\s\S]*?<\/script>/gi, '')
        .replace(/<style[\s\S]*?<\/style>/gi, '')
        .trim();
}

function getWhatsAppMediaUrl(template: Template): string {
    const sampleData = template.metadata?.sample_data as Record<string, unknown> | undefined;
    if (!sampleData || typeof sampleData !== 'object' || Array.isArray(sampleData)) return '';

    const value =
        sampleData.media_url ||
        sampleData.image_url ||
        sampleData.image ||
        sampleData.img_url ||
        '';

    return value == null ? '' : String(value);
}

function getWhatsAppMediaUrlFromDefaults(defaults: Record<string, string>): string {
    return (
        defaults.media_url ||
        defaults.image_url ||
        defaults.image ||
        defaults.img_url ||
        ''
    );
}

function applyDefaultsToTemplateBody(body: string, defaults: Record<string, string>): string {
    if (!body) return body;

    return body.replace(/{{\s*\.?([\w]+)\s*}}/g, (match, variable: string) => {
        if (Object.prototype.hasOwnProperty.call(defaults, variable)) {
            return defaults[variable] ?? '';
        }
        return match;
    });
}

function WhatsAppMobilePreview({
    rendered,
    mediaUrl,
    compact = false,
}: {
    rendered: string;
    mediaUrl?: string;
    compact?: boolean;
}) {
    const timeString = new Date().toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        hour12: true,
    });

    const applicationName = localStorage.getItem('last_app_name') || 'FreeRange Notify';
    const bubbleHtml = extractWhatsAppBody(rendered);
    const hasHtml = /<[^>]+>/.test(bubbleHtml);
    const showMedia = !!mediaUrl && /^(https?:)?\/\//i.test(mediaUrl);

    return (
        <div className={`h-full overflow-y-auto bg-muted/20 ${compact ? 'p-2' : 'p-4 sm:p-6'}`}>
            <div className="mx-auto flex h-full w-full max-w-sm flex-col overflow-hidden rounded-lg border border-border bg-[#efe7de] dark:bg-[#0b141a]">
                <div className="relative z-10 border-b border-black/10 bg-[#f0f2f5] px-3 py-2.5 dark:border-white/10 dark:bg-[#202c33]">
                    <div className="flex items-center gap-2.5">
                        <div className="flex size-9 items-center justify-center rounded-full bg-emerald-600 text-xl text-white">
                            {applicationName.substring(0, 1).toUpperCase()}
                        </div>
                        <div className="min-w-0 flex-1">
                            <p className="truncate text-sm font-semibold text-[#111b21] dark:text-[#e9edef]">{applicationName}</p>
                            <p className="text-[11px] text-[#667781] dark:text-[#8696a0]">Business Account</p>
                        </div>
                        {!compact && (
                            <div className="flex flex-row items-center gap-4">
                                <Video className="size-6 text-[#667781] dark:text-[#8696a0]" />
                                <Phone className="size-5 text-[#667781] dark:text-[#8696a0]" />
                                <MoreVertical className="size-5 text-[#667781] dark:text-[#8696a0]" />
                            </div>
                        )}
                    </div>
                </div>

                <div className="flex flex-1 flex-col px-3 py-3">
                    {!compact && (
                        <div className="mb-3 self-center rounded-md bg-white/80 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-[#667781] dark:bg-[#182229] dark:text-[#8696a0]">
                            Today
                        </div>
                    )}

                    {showMedia && (
                        <div className="relative mb-2 max-w-[86%] rounded-2xl rounded-tl-sm bg-white p-1.5 shadow dark:bg-[#202c33]">
                            <img src={mediaUrl} alt="WhatsApp media" className="max-h-56 w-full rounded-xl object-cover" />
                            <div className="mt-1 text-right text-[10px] text-[#667781] dark:text-[#8696a0]">{timeString}</div>
                        </div>
                    )}

                    <div className="relative max-w-[86%] rounded-2xl rounded-tl-sm bg-white px-3 py-2 text-[14px] leading-5 text-[#111b21] shadow dark:bg-[#202c33] dark:text-[#e9edef]">
                        {hasHtml ? (
                            <div
                                className="[&_a]:text-emerald-600 [&_a]:underline [&_p]:my-0 [&_strong]:font-semibold"
                                dangerouslySetInnerHTML={{ __html: bubbleHtml }}
                            />
                        ) : (
                            <p className="whitespace-pre-wrap">{bubbleHtml || 'No WhatsApp content rendered.'}</p>
                        )}
                        <div className="mt-1 text-right text-[10px] text-[#667781] dark:text-[#8696a0]">{timeString}</div>
                    </div>
                </div>

                {!compact && (
                    <div className="border-t border-black/10 bg-[#f0f2f5] p-2 dark:border-white/10 dark:bg-[#202c33]">
                        <div className="flex items-center gap-1.5">
                            <div className="flex flex-1 items-center gap-2 rounded-full bg-white px-3 py-2 text-[#667781] dark:bg-[#2a3942] dark:text-[#8696a0]">
                                <Smile className="h-4.5 w-4.5" />
                                <span className="flex-1 text-xs">Message</span>
                                <Paperclip className="h-4 w-4 -rotate-45" />
                                <Camera className="h-4 w-4" />
                            </div>
                            <div className="rounded-full bg-[#00a884] p-2 text-white">
                                <Mic className="h-4 w-4" />
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    );
}

interface TemplateLibraryCardProps {
    template: Template;
    category: string;
    cloning: string | null;
    defaultRenderData: Record<string, string>;
    renderedPreview?: string;
    previewLoading?: boolean;
    onOpenFullscreen: (template: Template) => void;
    onImport: (name: string) => void;
}

function TemplateLibraryCard({
    template,
    category,
    cloning,
    defaultRenderData,
    renderedPreview,
    previewLoading,
    onOpenFullscreen,
    onImport,
}: TemplateLibraryCardProps) {
    const channel = channelMeta[template.channel] || { icon: null, label: template.channel };
    const whatsappMediaUrl = getWhatsAppMediaUrlFromDefaults(defaultRenderData) || getWhatsAppMediaUrl(template);
    const previewContent = renderedPreview || applyDefaultsToTemplateBody(template.body, defaultRenderData);

    // remove underscores from name, and capitalize first letter of each word for display
    const templateName = template.name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase());

    const containerRef = useRef<HTMLDivElement>(null);
    const [scale, setScale] = useState(1);

    useEffect(() => {
        if (!containerRef.current) return;
        const observer = new ResizeObserver((entries) => {
            for (let entry of entries) {
                setScale(Math.min(1, entry.contentRect.width / 600));
            }
        });
        observer.observe(containerRef.current);
        return () => observer.disconnect();
    }, []);

    return (
        <Card className="group flex h-full flex-col overflow-hidden border-border/80 bg-card/70 shadow-sm transition-shadow hover:shadow-md">
            <CardContent className="flex flex-1 flex-col p-0">
                <div className="space-y-4 p-4">
                    <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1 space-y-2">
                            <div className="flex flex-wrap items-center gap-2">
                                <h3 className="truncate text-sm font-semibold tracking-tight">{templateName}</h3>
                                <Badge variant="outline" className={`text-[10px] leading-none ${categoryColors[category] || ''}`}>
                                    {formatCategory(category)}
                                </Badge>
                            </div>
                            <p className="line-clamp-2 pr-2 text-xs leading-5 text-muted-foreground">
                                {template.description || 'Ready-to-use template you can import and adapt.'}
                            </p>
                            {template.metadata?.usecase && (
                                <p className="mt-1.5 line-clamp-2 text-[11px] font-medium leading-relaxed text-blue-600/80 dark:text-blue-400/90">
                                    <strong className="font-semibold text-blue-700/80 dark:text-blue-300">Best for:</strong> {template.metadata.usecase as string}
                                </p>
                            )}
                        </div>
                        <div className="flex shrink-0 flex-col items-end gap-1.5">
                            <div className="flex items-center gap-2">
                                <Badge variant="outline" className="h-6 gap-1.5 px-2">
                                    <span className="text-muted-foreground">{channel.icon}</span>
                                    <span className="text-[11px]">{channel.label}</span>
                                </Badge>
                                <Button
                                    size="icon"
                                    variant="outline"
                                    className="h-6 w-6 border-primary/20 bg-primary/5 hover:bg-primary/20 transition-colors"
                                    title="Import Template"
                                    disabled={cloning === template.name}
                                    onClick={() => onImport(template.name)}
                                >
                                    {cloning === template.name ? <Loader2 className="h-3 w-3 animate-spin" /> : <Download className="h-3 w-3" />}
                                </Button>
                            </div>
                            <Badge variant="outline" className="h-6 px-2 text-[11px]">
                                {template.variables && template.variables.length > 0
                                    ? `${template.variables.length} field${template.variables.length === 1 ? '' : 's'}`
                                    : '0 fields'}
                            </Badge>
                        </div>
                    </div>
                </div>

                <div className="border-y border-border/70 bg-muted/20 px-4 py-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                        <p className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">Preview</p>
                        <div className="flex items-center gap-1">
                            <Button
                                size="sm"
                                variant="ghost"
                                className="h-6 px-2 text-xs opacity-90 transition-opacity group-hover:opacity-100"
                                onClick={() => onOpenFullscreen(template)}
                            >
                                <Expand className="h-3.5 w-3.5" />
                                Fullscreen
                            </Button>
                        </div>
                    </div>
                    {previewLoading ? (
                        <div className="flex h-56 items-center justify-center rounded-md border border-border bg-background text-muted-foreground">
                            <Loader2 className="h-4 w-4 animate-spin" />
                        </div>
                    ) : template.channel === 'email' ? (
                        <div ref={containerRef} className="relative w-full overflow-hidden rounded-md border border-border bg-white" style={{ height: `${900 * scale}px` }}>
                            <div className="absolute top-0 left-0 w-[600px] h-[900px] origin-top-left" style={{ transform: `scale(${scale})` }}>
                                <iframe
                                    srcDoc={previewContent}
                                    className="h-full w-full border-0 pointer-events-none"
                                    title={`Preview: ${template.name}`}
                                    sandbox=""
                                    scrolling="no"
                                />
                            </div>
                        </div>
                    ) : template.channel === 'whatsapp' ? (
                        <div className="h-56 overflow-hidden rounded-md border border-border bg-background">
                            <WhatsAppMobilePreview rendered={previewContent} mediaUrl={whatsappMediaUrl} compact />
                        </div>
                    ) : (
                        <div className="max-h-56 h-56 overflow-auto rounded-md border border-border bg-background p-3">
                            <pre className="text-xs font-mono whitespace-pre-wrap wrap-break-word text-foreground">{previewContent}</pre>
                        </div>
                    )}
                    <p className="mt-2 text-[11px] text-muted-foreground">
                        Preview uses default sample values.
                    </p>
                </div>
            </CardContent>
        </Card>
    );
}

export default function TemplateLibrary() {
    const { id: appId } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [apiKey, setApiKey] = useState('');
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [cloning, setCloning] = useState<string | null>(null);
    const [fullscreenTemplate, setFullscreenTemplate] = useState<Template | null>(null);
    const [renderedPreviews, setRenderedPreviews] = useState<Record<string, string>>({});
    const [previewLoading, setPreviewLoading] = useState<Record<string, boolean>>({});

    const getTemplatePreviewKey = (t: Template) => t.id || t.name;

    const getPreviewStorageKey = (templateId: string) =>
        `frn:template-preview-data:${appId}:${templateId}`;

    const getDefaultRenderData = (t: Template): Record<string, string> => {
        if (t.id && appId) {
            try {
                const persisted = localStorage.getItem(getPreviewStorageKey(t.id));
                if (persisted) {
                    const parsed = JSON.parse(persisted) as Record<string, unknown>;
                    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
                        return Object.fromEntries(
                            Object.entries(parsed).map(([key, value]) => [key, value == null ? '' : String(value)]),
                        );
                    }
                }
            } catch {
                // Ignore invalid or inaccessible local storage.
            }
        }

        const sampleData = t.metadata?.sample_data as Record<string, unknown> | undefined;
        if (sampleData && typeof sampleData === 'object' && !Array.isArray(sampleData)) {
            return Object.fromEntries(
                Object.entries(sampleData).map(([key, value]) => [key, value == null ? '' : String(value)]),
            );
        }

        const generated: Record<string, string> = {};
        for (const variable of t.variables || []) {
            generated[variable] = variable;
        }
        return generated;
    };

    const renderTemplatePreview = async (t: Template) => {
        if (!apiKey) return;
        const key = getTemplatePreviewKey(t);

        if (renderedPreviews[key] || previewLoading[key]) return;

        setPreviewLoading((prev) => ({ ...prev, [key]: true }));
        try {
            const payload = {
                data: getDefaultRenderData(t),
                editable: false,
            };
            const response = t.id
                ? await templatesAPI.render(apiKey, t.id, payload)
                : t.name
                    ? await templatesAPI.renderLibrary(apiKey, t.name, payload)
                    : null;
            setRenderedPreviews((prev) => ({
                ...prev,
                [key]: response?.rendered_body || t.body,
            }));
        } catch {
            setRenderedPreviews((prev) => ({ ...prev, [key]: t.body }));
        } finally {
            setPreviewLoading((prev) => ({ ...prev, [key]: false }));
        }
    };

    useEffect(() => {
        if (!appId) return;
        (async () => {
            try {
                const app = await applicationsAPI.get(appId);
                const key = app.api_key || '';
                setApiKey(key);
                if (key) {
                    const res = await templatesAPI.getLibrary(key);
                    setTemplates(res.templates || []);
                }
            } catch {
                toast.error('Failed to load template library');
            } finally {
                setLoading(false);
            }
        })();
    }, [appId]);

    useEffect(() => {
        if (!apiKey || templates.length === 0) return;

        let cancelled = false;
        const run = async () => {
            for (const t of templates) {
                if (cancelled) return;
                await renderTemplatePreview(t);
            }
        };

        run();
        return () => {
            cancelled = true;
        };
    }, [apiKey, templates]);

    const handleImport = async (name: string) => {
        if (!apiKey) return;
        setCloning(name);
        try {
            await templatesAPI.cloneFromLibrary(apiKey, name);
            toast.success(`Template "${name}" imported successfully!`);
            navigate(`/apps/${appId}?tab=templates`);
        } catch (error: any) {
            const msg = error?.response?.data?.message || error?.message || 'Import failed';
            toast.error(msg);
        } finally {
            setCloning(null);
        }
    };

    const getCategory = (t: Template) =>
        (t.metadata?.category as string) || 'notification';

    const [activeChannel, setActiveChannel] = useState<string>('email');

    const CHANNELS = [
        { id: 'email', label: 'Email', icon: <Mail className="w-4 h-4 mr-2" /> },
        { id: 'sms', label: 'SMS', icon: <MessageSquare className="w-4 h-4 mr-2" /> },
        { id: 'whatsapp', label: 'WhatsApp', icon: <Phone className="w-4 h-4 mr-2" /> },
        { id: 'push', label: 'Push', icon: <Bell className="w-4 h-4 mr-2" /> },
        { id: 'webhook', label: 'Webhook', icon: <Webhook className="w-4 h-4 mr-2" /> },
        { id: 'sse', label: 'SSE', icon: <Radio className="w-4 h-4 mr-2" /> },
        { id: 'in_app', label: 'In-App', icon: <Globe className="w-4 h-4 mr-2" /> },
    ];

    const filteredTemplates = useMemo(() => {
        const orderWeight: Record<string, number> = {
            'welcome_email': 100,
            'password_reset': 100,
            'order_confirmation': 100,
            'appointment_reminder': 100,
            'booking_confirmation': 100,
            'maintenance_notice': 100,
        };
        const list = templates.filter((t) => t.channel === activeChannel);
        return list.sort((a, b) => {
            const weightA = orderWeight[a.name] || 0;
            const weightB = orderWeight[b.name] || 0;
            return weightA - weightB;
        });
    }, [templates, activeChannel]);

    const defaultRenderDataByTemplate = useMemo(() => {
        const result: Record<string, Record<string, string>> = {};
        for (const t of templates) {
            result[getTemplatePreviewKey(t)] = getDefaultRenderData(t);
        }
        return result;
    }, [templates, appId]);

    const fullscreenPreviewKey = fullscreenTemplate ? getTemplatePreviewKey(fullscreenTemplate) : null;
    const fullscreenPreviewLoading = fullscreenPreviewKey ? !!previewLoading[fullscreenPreviewKey] : false;
    const fullscreenDefaultRenderData = fullscreenPreviewKey ? defaultRenderDataByTemplate[fullscreenPreviewKey] || {} : {};
    const fullscreenRenderedPreview = fullscreenPreviewKey
        ? renderedPreviews[fullscreenPreviewKey] || applyDefaultsToTemplateBody(fullscreenTemplate?.body || '', fullscreenDefaultRenderData)
        : '';

    return (
        <>
            <div className="space-y-6">
                <Card size="sm" className="bg-card/60 shadow-sm">
                    <CardHeader className="py-4">
                        <div className="flex items-center gap-3">
                            <Button variant="outline" size="sm" className='items-center font-medium shadow-sm' onClick={() => navigate(`/apps/${appId}?tab=templates`)}>
                                <ArrowLeft className="mr-2 h-4 w-4" />
                                Back to Templates
                            </Button>
                            <div>
                                <CardTitle className="text-xl">Template Library</CardTitle>
                                <p className="text-sm text-muted-foreground hidden sm:block">
                                    Browse by channel, preview with data, and import directly to your app.
                                </p>
                            </div>
                        </div>
                    </CardHeader>
                </Card>

                <div className="flex flex-col md:flex-row gap-6 items-start">
                    {/* Channel Sidebar */}
                    <aside className="w-full md:w-56 shrink-0 sticky top-4">
                        <Card className="bg-card/60 shadow-sm border-border/80">
                            <CardContent className="p-2 space-y-1">
                                {CHANNELS.map(ch => {
                                    const count = templates.filter(t => t.channel === ch.id).length;
                                    if (count === 0) return null; // Only show active channels
                                    return (
                                        <button
                                            key={ch.id}
                                            onClick={() => setActiveChannel(ch.id)}
                                            className={`w-full flex items-center justify-start text-sm px-3 py-2.5 rounded-md transition-colors ${
                                                activeChannel === ch.id
                                                    ? 'bg-foreground text-background font-medium shadow-sm'
                                                    : 'text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                                            }`}
                                        >
                                            {ch.icon}
                                            {ch.label}
                                            <Badge variant={activeChannel === ch.id ? "secondary" : "outline"} className={`ml-auto text-[10px] ${activeChannel === ch.id ? 'bg-background/20 text-background hover:bg-background/20' : ''}`}>
                                                {count}
                                            </Badge>
                                        </button>
                                    );
                                })}
                            </CardContent>
                        </Card>
                    </aside>

                    {/* Template Grid */}
                    <div className="flex-1 min-w-0">
                        {loading ? (
                            <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                                {Array.from({ length: 4 }).map((_, i) => (
                                    <Card key={i} className="overflow-hidden border-border/80">
                                        <CardContent className="space-y-3 p-4">
                                            <Skeleton className="h-4 w-3/4" />
                                            <Skeleton className="h-3 w-full" />
                                            <Skeleton className="h-56 w-full rounded-md" />
                                        </CardContent>
                                    </Card>
                                ))}
                            </div>
                        ) : filteredTemplates.length === 0 ? (
                            <Card className="border-border/80">
                                <CardContent className="py-14 text-center text-sm text-muted-foreground">
                                    No {activeChannel} templates available in the library yet.
                                </CardContent>
                            </Card>
                        ) : (
                            <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
                                {filteredTemplates.map((t) => (
                                    <TemplateLibraryCard
                                        key={t.name}
                                        template={t}
                                        category={getCategory(t)}
                                        cloning={cloning}
                                        defaultRenderData={defaultRenderDataByTemplate[getTemplatePreviewKey(t)] || {}}
                                        renderedPreview={renderedPreviews[getTemplatePreviewKey(t)]}
                                        previewLoading={previewLoading[getTemplatePreviewKey(t)]}
                                        onOpenFullscreen={(template) => setFullscreenTemplate(template)}
                                        onImport={handleImport}
                                    />
                                ))}
                            </div>
                        )}
                    </div>
                </div>
            </div>

            <SlidePanel
                open={!!fullscreenTemplate}
                onClose={() => setFullscreenTemplate(null)}
                title={fullscreenTemplate ? `Preview: ${fullscreenTemplate.name}` : 'Preview'}
                size="full"
            >
                {fullscreenTemplate && (
                    <div className="flex h-full flex-col gap-3">
                        <div className="flex flex-wrap items-center gap-2 rounded-md border border-border/70 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                            <span>Rendered with default sample values.</span>
                            <Badge variant="outline" className="text-[10px]">
                                {channelMeta[fullscreenTemplate.channel]?.label || fullscreenTemplate.channel}
                            </Badge>
                            {fullscreenTemplate.channel === 'email' && fullscreenTemplate.subject && (
                                <Badge variant="outline" className="max-w-full text-[10px]">
                                    Subject: {fullscreenTemplate.subject}
                                </Badge>
                            )}
                        </div>

                        <div className="min-h-0 flex-1 rounded-md border border-border/80 bg-background p-3">
                            {fullscreenPreviewLoading ? (
                                <div className="flex h-full items-center justify-center text-muted-foreground">
                                    <Loader2 className="h-5 w-5 animate-spin" />
                                </div>
                            ) : fullscreenTemplate.channel === 'email' ? (
                                <iframe
                                    srcDoc={fullscreenRenderedPreview}
                                    className="h-full min-h-90 w-full rounded-md border-0 bg-white"
                                    title={`Fullscreen Preview: ${fullscreenTemplate.name}`}
                                    sandbox=""
                                />
                            ) : fullscreenTemplate.channel === 'whatsapp' ? (
                                <WhatsAppMobilePreview
                                    rendered={fullscreenRenderedPreview}
                                    mediaUrl={getWhatsAppMediaUrlFromDefaults(fullscreenDefaultRenderData) || getWhatsAppMediaUrl(fullscreenTemplate)}
                                />
                            ) : (
                                <div className="h-full overflow-auto rounded-md border border-border bg-muted/15 p-4">
                                    <pre className="text-sm font-mono whitespace-pre-wrap wrap-break-word text-foreground">
                                        {fullscreenRenderedPreview}
                                    </pre>
                                </div>
                            )}
                        </div>
                    </div>
                )}
            </SlidePanel>
        </>
    );
}
