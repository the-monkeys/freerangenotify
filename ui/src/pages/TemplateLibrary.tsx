import { useEffect, useMemo, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { applicationsAPI, templatesAPI } from '../services/api';
import type { Template } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Skeleton } from '../components/ui/skeleton';
import { SlidePanel } from '../components/ui/slide-panel';
import { toast } from 'sonner';
import { ArrowLeft, Download, Expand, Eye, EyeOff, Loader2, Mail, Bell, MessageSquare, Globe, Radio, Webhook } from 'lucide-react';

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

interface TemplateLibraryCardProps {
    template: Template;
    category: string;
    expandedPreview: string | null;
    cloning: string | null;
    renderedPreview?: string;
    previewLoading?: boolean;
    onTogglePreview: (name: string) => void;
    onOpenFullscreen: (template: Template) => void;
    onImport: (name: string) => void;
}

function TemplateLibraryCard({
    template,
    category,
    expandedPreview,
    cloning,
    renderedPreview,
    previewLoading,
    onTogglePreview,
    onOpenFullscreen,
    onImport,
}: TemplateLibraryCardProps) {
    const isExpanded = expandedPreview === template.name;
    const channel = channelMeta[template.channel] || { icon: null, label: template.channel };

    return (
        <Card className="flex h-full flex-col overflow-hidden border-border/80 bg-card/70 shadow-sm">
            <CardContent className="flex flex-1 flex-col p-0">
                <div className="space-y-3 p-4">
                    <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0 space-y-1">
                            <div className="flex flex-wrap items-center gap-1.5">
                                <h3 className="truncate text-sm font-semibold">{template.name}</h3>
                                <Badge variant="outline" className={`text-[10px] ${categoryColors[category] || ''}`}>
                                    {formatCategory(category)}
                                </Badge>
                            </div>
                            <p className="text-xs text-muted-foreground line-clamp-2">
                                {template.description || 'Ready-to-use template you can import and adapt.'}
                            </p>
                        </div>
                        <div className="inline-flex items-center gap-1 rounded-md border border-border/80 bg-background px-2 py-1 text-[11px] text-muted-foreground">
                            {channel.icon}
                            <span>{channel.label}</span>
                        </div>
                    </div>

                    {template.subject && (
                        <div className="rounded-md border border-border/70 bg-muted/25 px-2.5 py-1.5 text-xs text-muted-foreground">
                            <span className="font-medium text-foreground">Subject:</span> {template.subject}
                        </div>
                    )}

                    <div className="rounded-md border border-border/70 bg-muted/25 px-2.5 py-1.5 text-xs text-muted-foreground">
                        {template.variables && template.variables.length > 0
                            ? `Personalized with ${template.variables.length} fields`
                            : 'No personalization fields required'}
                    </div>
                </div>

                <div className="border-y border-border/70 bg-muted/20 px-4 py-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                        <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">Preview</p>
                        <div className="flex items-center gap-1">
                            <Button
                                size="sm"
                                variant="ghost"
                                className="h-6 px-2 text-xs"
                                onClick={() => onTogglePreview(template.name)}
                            >
                                {isExpanded ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
                                {isExpanded ? 'Compact' : 'Large'}
                            </Button>
                            <Button
                                size="sm"
                                variant="ghost"
                                className="h-6 px-2 text-xs"
                                onClick={() => onOpenFullscreen(template)}
                            >
                                <Expand className="h-3.5 w-3.5" />
                                Fullscreen
                            </Button>
                        </div>
                    </div>
                    {previewLoading ? (
                        <div className={`flex items-center justify-center rounded-md border border-border bg-background text-muted-foreground ${isExpanded ? 'h-72' : 'h-44'}`}>
                            <Loader2 className="h-4 w-4 animate-spin" />
                        </div>
                    ) : template.channel === 'email' ? (
                        <iframe
                            srcDoc={renderedPreview || template.body}
                            className={`w-full rounded-md border-0 bg-white ${isExpanded ? 'h-72' : 'h-44'}`}
                            title={`Preview: ${template.name}`}
                            sandbox=""
                        />
                    ) : (
                        <div className={`overflow-auto rounded-md border border-border bg-background p-3 ${isExpanded ? 'max-h-80' : 'max-h-44'}`}>
                            <pre className="text-xs font-mono whitespace-pre-wrap wrap-break-word text-foreground">{renderedPreview || template.body}</pre>
                        </div>
                    )}
                    <p className="mt-2 text-[11px] text-muted-foreground">
                        Preview uses default sample values.
                    </p>
                </div>

                <div className="mt-auto p-4 pt-3">
                    <Button
                        size="sm"
                        className="w-full"
                        disabled={cloning === template.name}
                        onClick={() => onImport(template.name)}
                    >
                        {cloning === template.name ? (
                            'Importing...'
                        ) : (
                            <>
                                <Download className="mr-1.5 h-3.5 w-3.5" />
                                Import Template
                            </>
                        )}
                    </Button>
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
    const [expandedPreview, setExpandedPreview] = useState<string | null>(null);
    const [fullscreenTemplate, setFullscreenTemplate] = useState<Template | null>(null);
    const [renderedPreviews, setRenderedPreviews] = useState<Record<string, string>>({});
    const [previewLoading, setPreviewLoading] = useState<Record<string, boolean>>({});

    const getTemplatePreviewKey = (t: Template) => t.id || t.name;

    const titleCase = (value: string) =>
        value
            .replace(/[._-]+/g, ' ')
            .trim()
            .replace(/\b\w/g, (c) => c.toUpperCase());

    const getDefaultRenderData = (t: Template): Record<string, string> => {
        const sampleData = t.metadata?.sample_data as Record<string, unknown> | undefined;
        if (sampleData && typeof sampleData === 'object' && !Array.isArray(sampleData)) {
            return Object.fromEntries(
                Object.entries(sampleData).map(([key, value]) => [key, value == null ? '' : String(value)]),
            );
        }

        const generated: Record<string, string> = {};
        for (const variable of t.variables || []) {
            generated[variable] = titleCase(variable) || variable;
        }
        return generated;
    };

    const renderTemplatePreview = async (t: Template) => {
        if (!apiKey || !t.id) return;
        const key = getTemplatePreviewKey(t);

        if (renderedPreviews[key] || previewLoading[key]) return;

        setPreviewLoading((prev) => ({ ...prev, [key]: true }));
        try {
            const response = await templatesAPI.render(apiKey, t.id, {
                data: getDefaultRenderData(t),
                editable: false,
            });
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

    const groupedTemplates = templates.reduce<Record<string, Template[]>>((acc, t) => {
        const cat = getCategory(t);
        (acc[cat] ||= []).push(t);
        return acc;
    }, {});

    const categoryOrder = ['transactional', 'newsletter', 'notification'];
    const sortedCategories = useMemo(
        () =>
            Object.keys(groupedTemplates).sort(
                (a, b) =>
                    (categoryOrder.indexOf(a) === -1 ? 99 : categoryOrder.indexOf(a)) -
                    (categoryOrder.indexOf(b) === -1 ? 99 : categoryOrder.indexOf(b)),
            ),
        [groupedTemplates],
    );

    const fullscreenPreviewKey = fullscreenTemplate ? getTemplatePreviewKey(fullscreenTemplate) : null;
    const fullscreenPreviewLoading = fullscreenPreviewKey ? !!previewLoading[fullscreenPreviewKey] : false;
    const fullscreenRenderedPreview = fullscreenPreviewKey
        ? renderedPreviews[fullscreenPreviewKey] || fullscreenTemplate?.body || ''
        : '';

    return (
        <>
            <div className="mx-auto max-w-7xl space-y-6">
                <Card size="sm" className="bg-card/60 shadow-sm">
                    <CardHeader className="">
                        <div className="flex items-center gap-2">
                            <Button variant="ghost" size="sm" className='items-center' onClick={() => navigate(`/apps/${appId}?tab=templates`)}>
                                <ArrowLeft className="size-4" />
                            </Button>
                            <div>
                                <CardTitle className="text-xl">Template Library</CardTitle>
                                <p className="text-sm text-muted-foreground">
                                    Choose by what each message looks like, then import and customize for your app.
                                </p>
                            </div>
                        </div>
                    </CardHeader>
                </Card>

                {loading ? (
                    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                        {Array.from({ length: 6 }).map((_, i) => (
                            <Card key={i} className="overflow-hidden border-border/80">
                                <CardContent className="space-y-3 p-4">
                                    <Skeleton className="h-4 w-3/4" />
                                    <Skeleton className="h-3 w-full" />
                                    <Skeleton className="h-3 w-5/6" />
                                    <Skeleton className="h-44 w-full rounded-md" />
                                    <Skeleton className="h-8 w-full" />
                                </CardContent>
                            </Card>
                        ))}
                    </div>
                ) : templates.length === 0 ? (
                    <Card className="border-border/80">
                        <CardContent className="py-14 text-center text-sm text-muted-foreground">
                            No templates available in the library.
                        </CardContent>
                    </Card>
                ) : (
                    sortedCategories.map((category) => (
                        <section key={category} className="space-y-3">
                            <div className="flex items-center gap-2">
                                <h2 className="text-base font-semibold">{formatCategory(category)}</h2>
                                <Badge variant="outline" className="text-xs font-normal">
                                    {groupedTemplates[category].length}
                                </Badge>
                            </div>

                            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                                {groupedTemplates[category].map((t) => (
                                    <TemplateLibraryCard
                                        key={t.name}
                                        template={t}
                                        category={category}
                                        expandedPreview={expandedPreview}
                                        cloning={cloning}
                                        renderedPreview={renderedPreviews[getTemplatePreviewKey(t)]}
                                        previewLoading={previewLoading[getTemplatePreviewKey(t)]}
                                        onTogglePreview={(name) =>
                                            setExpandedPreview((prev) => (prev === name ? null : name))
                                        }
                                        onOpenFullscreen={(template) => setFullscreenTemplate(template)}
                                        onImport={handleImport}
                                    />
                                ))}
                            </div>
                        </section>
                    ))
                )}
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
