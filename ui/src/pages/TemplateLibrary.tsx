import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { applicationsAPI, templatesAPI } from '../services/api';
import type { Template } from '../types';
import { Button } from '../components/ui/button';
import { Card, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Skeleton } from '../components/ui/skeleton';
import { toast } from 'sonner';
import { ArrowLeft, Download, Eye, EyeOff, Mail, Bell, MessageSquare, Globe, Radio, Webhook } from 'lucide-react';

const channelIcon: Record<string, React.ReactNode> = {
    email: <Mail className="h-4 w-4" />,
    push: <Bell className="h-4 w-4" />,
    sms: <MessageSquare className="h-4 w-4" />,
    webhook: <Webhook className="h-4 w-4" />,
    in_app: <Globe className="h-4 w-4" />,
    sse: <Radio className="h-4 w-4" />,
};

const categoryColors: Record<string, string> = {
    transactional: 'bg-blue-500/10 text-blue-600 border-blue-500/20',
    newsletter: 'bg-purple-500/10 text-purple-600 border-purple-500/20',
    notification: 'bg-amber-500/10 text-amber-600 border-amber-500/20',
};

export default function TemplateLibrary() {
    const { id: appId } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [apiKey, setApiKey] = useState('');
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [cloning, setCloning] = useState<string | null>(null);
    const [expandedPreview, setExpandedPreview] = useState<string | null>(null);

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
    const sortedCategories = Object.keys(groupedTemplates).sort(
        (a, b) => (categoryOrder.indexOf(a) === -1 ? 99 : categoryOrder.indexOf(a)) - (categoryOrder.indexOf(b) === -1 ? 99 : categoryOrder.indexOf(b))
    );

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center gap-4">
                <Button variant="ghost" size="icon" onClick={() => navigate(`/apps/${appId}?tab=templates`)}>
                    <ArrowLeft className="h-5 w-5" />
                </Button>
                <div>
                    <h1 className="text-2xl font-bold">Template Library</h1>
                    <p className="text-sm text-muted-foreground">
                        Pre-built templates ready to import into your app. Preview and customise after importing.
                    </p>
                </div>
            </div>

            {loading ? (
                <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                    {Array.from({ length: 6 }).map((_, i) => (
                        <Skeleton key={i} className="h-48 rounded-lg" />
                    ))}
                </div>
            ) : templates.length === 0 ? (
                <div className="text-center py-16 text-muted-foreground">
                    No templates available in the library.
                </div>
            ) : (
                sortedCategories.map(category => (
                    <div key={category} className="space-y-3">
                        <h2 className="text-lg font-semibold capitalize flex items-center gap-2">
                            {category}
                            <Badge variant="outline" className="text-xs font-normal">{groupedTemplates[category].length}</Badge>
                        </h2>
                        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
                            {groupedTemplates[category].map(t => (
                                <Card key={t.name} className="flex flex-col">
                                    <CardContent className="flex flex-col flex-1 p-5 space-y-3">
                                        {/* Header row */}
                                        <div className="flex items-start justify-between gap-2">
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2 flex-wrap">
                                                    <h3 className="font-semibold text-sm">{t.name}</h3>
                                                    <Badge variant="outline" className={`text-[10px] ${categoryColors[getCategory(t)] || ''}`}>
                                                        {getCategory(t)}
                                                    </Badge>
                                                </div>
                                                <p className="text-xs text-muted-foreground mt-1 line-clamp-2">{t.description}</p>
                                            </div>
                                            <div className="flex items-center gap-1.5 text-muted-foreground shrink-0">
                                                {channelIcon[t.channel] || null}
                                                <span className="text-xs">{t.channel}</span>
                                            </div>
                                        </div>

                                        {/* Subject */}
                                        {t.subject && (
                                            <div className="text-xs">
                                                <span className="text-muted-foreground">Subject: </span>
                                                <span className="font-mono">{t.subject}</span>
                                            </div>
                                        )}

                                        {/* Variables */}
                                        {t.variables && t.variables.length > 0 && (
                                            <div className="flex flex-wrap gap-1">
                                                {t.variables.map(v => (
                                                    <Badge key={v} variant="secondary" className="text-[10px] font-mono">
                                                        {'{{.' + v + '}}'}
                                                    </Badge>
                                                ))}
                                            </div>
                                        )}

                                        {/* Preview toggle */}
                                        <div className="flex-1">
                                            <button
                                                className="text-xs text-muted-foreground hover:text-foreground flex items-center gap-1 mb-2"
                                                onClick={() => setExpandedPreview(expandedPreview === t.name ? null : t.name)}
                                            >
                                                {expandedPreview === t.name ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                                                {expandedPreview === t.name ? 'Hide preview' : 'Show preview'}
                                            </button>
                                            {expandedPreview === t.name && (
                                                <div className="border rounded-md bg-muted p-3 max-h-[300px] overflow-auto">
                                                    {t.channel === 'email' ? (
                                                        <iframe
                                                            srcDoc={t.body}
                                                            className="w-full h-[250px] border-0 rounded bg-white"
                                                            title={`Preview: ${t.name}`}
                                                            sandbox=""
                                                        />
                                                    ) : (
                                                        <pre className="text-xs font-mono whitespace-pre-wrap break-words">{t.body}</pre>
                                                    )}
                                                </div>
                                            )}
                                        </div>

                                        {/* Import button */}
                                        <Button
                                            size="sm"
                                            className="w-full"
                                            disabled={cloning === t.name}
                                            onClick={() => handleImport(t.name)}
                                        >
                                            {cloning === t.name ? (
                                                'Importing...'
                                            ) : (
                                                <>
                                                    <Download className="h-3.5 w-3.5 mr-1.5" />
                                                    Import Template
                                                </>
                                            )}
                                        </Button>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    </div>
                ))
            )}
        </div>
    );
}
