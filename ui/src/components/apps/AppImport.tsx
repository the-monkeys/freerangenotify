import React, { useEffect, useState } from 'react';
import { applicationsAPI } from '../../services/api';
import { useAuth } from '../../contexts/AuthContext';
import type { Application } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Checkbox } from '../ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Badge } from '../ui/badge';
import { toast } from 'sonner';
import { Link2, Unlink, Loader2, ArrowRight, Trash2, ShieldAlert } from 'lucide-react';

const RESOURCE_TYPES = [
    { id: 'users', label: 'Users', description: 'Subscriber users and their preferences' },
    { id: 'templates', label: 'Templates', description: 'Notification templates across all channels' },
    { id: 'workflows', label: 'Workflows', description: 'Automation workflows and triggers' },
    { id: 'digest_rules', label: 'Digest Rules', description: 'Notification batching/digest configurations' },
    { id: 'topics', label: 'Topics', description: 'Topic subscriptions and subscriber groups' },
    { id: 'providers', label: 'Providers', description: 'Custom notification provider configurations' },
];

interface AppImportProps {
    appId: string;
    appName: string;
}

interface LinkRecord {
    link_id: string;
    source_app_id: string;
    resource_type: string;
    resource_id: string;
    linked_at: string;
}

const AppImport: React.FC<AppImportProps> = ({ appId, appName }) => {
    const { user } = useAuth();
    const [apps, setApps] = useState<Application[]>([]);
    const [sourceAppId, setSourceAppId] = useState('');
    const [selectedTypes, setSelectedTypes] = useState<string[]>([]);
    const [importing, setImporting] = useState(false);
    const [links, setLinks] = useState<LinkRecord[]>([]);
    const [loadingLinks, setLoadingLinks] = useState(true);

    useEffect(() => {
        loadApps();
        loadLinks();
    }, [appId]);

    const loadApps = async () => {
        try {
            const allApps = await applicationsAPI.list();
            // Only show apps the current user owns (admin_user_id matches)
            // to prevent unauthorized cross-app access attempts
            const ownedApps = allApps.filter((a: Application) =>
                a.app_id !== appId && a.admin_user_id === user?.user_id
            );
            setApps(ownedApps);
        } catch { /* ignore */ }
    };

    const loadLinks = async () => {
        setLoadingLinks(true);
        try {
            const result = await applicationsAPI.listLinks(appId);
            setLinks(result.links || []);
        } catch { /* ignore */ }
        finally { setLoadingLinks(false); }
    };

    const toggleType = (id: string) => {
        setSelectedTypes(prev =>
            prev.includes(id) ? prev.filter(t => t !== id) : [...prev, id]
        );
    };

    const handleImport = async () => {
        if (!sourceAppId || selectedTypes.length === 0) {
            toast.error('Select a source app and at least one resource type');
            return;
        }
        setImporting(true);
        try {
            const result = await applicationsAPI.importResources(appId, sourceAppId, selectedTypes);
            const linkedTotal = Object.values(result.linked).reduce((a, b) => a + b, 0);
            const skippedTotal = Object.values(result.skipped).reduce((a, b) => a + b, 0);
            toast.success(`Linked ${linkedTotal} resource(s)${skippedTotal > 0 ? `, ${skippedTotal} already linked` : ''}`);
            setSelectedTypes([]);
            loadLinks();
        } catch (err) {
            toast.error(extractErrorMessage(err, 'Import failed'));
        } finally {
            setImporting(false);
        }
    };

    const handleUnlink = async (linkId: string) => {
        try {
            await applicationsAPI.removeLink(appId, linkId);
            setLinks(prev => prev.filter(l => l.link_id !== linkId));
            toast.success('Link removed');
        } catch (err) {
            toast.error(extractErrorMessage(err, 'Failed to remove link'));
        }
    };

    const handleUnlinkAll = async () => {
        if (!window.confirm('Remove all resource links? Resources in the source app are not affected.')) return;
        try {
            await applicationsAPI.removeAllLinks(appId);
            setLinks([]);
            toast.success('All links removed');
        } catch (err) {
            toast.error(extractErrorMessage(err, 'Failed to remove links'));
        }
    };

    const sourceApp = apps.find(a => a.app_id === sourceAppId);

    const linksBySource = links.reduce<Record<string, LinkRecord[]>>((acc, l) => {
        (acc[l.source_app_id] = acc[l.source_app_id] || []).push(l);
        return acc;
    }, {});

    const linksByType = links.reduce<Record<string, number>>((acc, l) => {
        acc[l.resource_type] = (acc[l.resource_type] || 0) + 1;
        return acc;
    }, {});

    return (
        <div className="space-y-6">
            {/* Import Section */}
            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Link2 className="h-5 w-5" />
                        Import Resources from Another App
                    </CardTitle>
                    <p className="text-sm text-muted-foreground mt-1">
                        Link resources from a source app into <strong>{appName}</strong>. Linked resources appear as read-only — no data is duplicated.
                    </p>
                </CardHeader>
                <CardContent className="space-y-4">
                    <div className="flex items-start gap-2 p-3 rounded-lg bg-muted/50 text-sm text-muted-foreground">
                        <ShieldAlert className="h-4 w-4 mt-0.5 shrink-0" />
                        <span>Only applications you own are shown below. You must have admin or owner access on both the source and target app to import resources.</span>
                    </div>

                    {/* Source App Selector */}
                    <div>
                        <label className="text-sm font-medium mb-1.5 block">Source Application</label>
                        {apps.length === 0 ? (
                            <p className="text-sm text-muted-foreground">No other apps found. Create another app first to link resources.</p>
                        ) : (
                            <Select value={sourceAppId} onValueChange={setSourceAppId}>
                                <SelectTrigger className="w-full max-w-md">
                                    <SelectValue placeholder="Select an app to import from..." />
                                </SelectTrigger>
                                <SelectContent>
                                    {apps.map(a => (
                                        <SelectItem key={a.app_id} value={a.app_id}>{a.app_name}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        )}
                    </div>

                    {/* Resource Type Checkboxes */}
                    {sourceAppId && (
                        <div>
                            <label className="text-sm font-medium mb-2 block">Resource Types to Link</label>
                            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                                {RESOURCE_TYPES.map(rt => (
                                    <label
                                        key={rt.id}
                                        className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${selectedTypes.includes(rt.id) ? 'border-primary bg-primary/5' : 'border-border hover:border-muted-foreground/30'}`}
                                    >
                                        <Checkbox
                                            checked={selectedTypes.includes(rt.id)}
                                            onCheckedChange={() => toggleType(rt.id)}
                                            className="mt-0.5"
                                        />
                                        <div>
                                            <div className="font-medium text-sm">{rt.label}</div>
                                            <div className="text-xs text-muted-foreground">{rt.description}</div>
                                        </div>
                                    </label>
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Import Button */}
                    {sourceAppId && selectedTypes.length > 0 && (
                        <div className="flex items-center gap-3 pt-2">
                            <Button onClick={handleImport} disabled={importing}>
                                {importing ? (
                                    <><Loader2 className="h-4 w-4 animate-spin mr-2" /> Linking...</>
                                ) : (
                                    <><span className="font-semibold">{sourceApp?.app_name}</span>
                                        <ArrowRight className="h-4 w-4 mx-1" />
                                        <span className="font-semibold">{appName}</span>
                                        <span className="ml-2">({selectedTypes.length} type{selectedTypes.length > 1 ? 's' : ''})</span>
                                    </>
                                )}
                            </Button>
                        </div>
                    )}
                </CardContent>
            </Card>

            {/* Active Links Section */}
            <Card>
                <CardHeader className="flex flex-row items-center justify-between">
                    <div>
                        <CardTitle className="flex items-center gap-2">
                            Active Links
                            {links.length > 0 && <Badge variant="secondary">{links.length}</Badge>}
                        </CardTitle>
                        <p className="text-sm text-muted-foreground mt-1">
                            Resources currently linked into this app from other apps.
                        </p>
                    </div>
                    {links.length > 0 && (
                        <Button variant="outline" size="sm" onClick={handleUnlinkAll} className="text-destructive hover:text-destructive">
                            <Trash2 className="h-3.5 w-3.5 mr-1" /> Remove All
                        </Button>
                    )}
                </CardHeader>
                <CardContent>
                    {loadingLinks ? (
                        <div className="flex items-center gap-2 text-muted-foreground py-4">
                            <Loader2 className="h-4 w-4 animate-spin" /> Loading links...
                        </div>
                    ) : links.length === 0 ? (
                        <p className="text-muted-foreground py-4">No linked resources yet. Use the import section above to link resources from another app.</p>
                    ) : (
                        <div className="space-y-4">
                            {/* Summary badges */}
                            <div className="flex flex-wrap gap-2">
                                {Object.entries(linksByType).map(([type, count]) => (
                                    <Badge key={type} variant="outline" className="text-xs">
                                        {type}: {count}
                                    </Badge>
                                ))}
                            </div>

                            {/* Links grouped by source app */}
                            {Object.entries(linksBySource).map(([srcAppId, srcLinks]) => {
                                const srcApp = apps.find(a => a.app_id === srcAppId);
                                const typeCount = srcLinks.reduce<Record<string, number>>((acc, l) => {
                                    acc[l.resource_type] = (acc[l.resource_type] || 0) + 1;
                                    return acc;
                                }, {});

                                return (
                                    <div key={srcAppId} className="border rounded-lg p-4">
                                        <div className="flex items-center justify-between mb-2">
                                            <div className="font-medium">
                                                From: <span className="text-primary">{srcApp?.app_name || srcAppId.slice(0, 8) + '...'}</span>
                                            </div>
                                            <span className="text-xs text-muted-foreground">{srcLinks.length} link(s)</span>
                                        </div>
                                        <div className="flex flex-wrap gap-2 mb-3">
                                            {Object.entries(typeCount).map(([type, count]) => (
                                                <Badge key={type} variant="secondary" className="text-xs">
                                                    {type} ({count})
                                                </Badge>
                                            ))}
                                        </div>
                                        <details className="text-sm">
                                            <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
                                                Show individual links
                                            </summary>
                                            <div className="mt-2 max-h-48 overflow-y-auto space-y-1">
                                                {srcLinks.map(link => (
                                                    <div key={link.link_id} className="flex items-center justify-between py-1 px-2 rounded hover:bg-muted/50">
                                                        <div className="flex items-center gap-2 text-xs">
                                                            <Badge variant="outline" className="text-[10px]">{link.resource_type}</Badge>
                                                            <span className="font-mono text-muted-foreground">{link.resource_id.slice(0, 12)}...</span>
                                                        </div>
                                                        <Button
                                                            variant="ghost"
                                                            size="sm"
                                                            className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                                                            onClick={() => handleUnlink(link.link_id)}
                                                        >
                                                            <Unlink className="h-3 w-3" />
                                                        </Button>
                                                    </div>
                                                ))}
                                            </div>
                                        </details>
                                    </div>
                                );
                            })}
                        </div>
                    )}
                </CardContent>
            </Card>
        </div>
    );
};

export default AppImport;
