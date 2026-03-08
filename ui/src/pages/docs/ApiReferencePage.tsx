import React, { useEffect, useState, useMemo } from 'react';
import { Input } from '../../components/ui/input';
import { Badge } from '../../components/ui/badge';
import { Skeleton } from '../../components/ui/skeleton';
import { ChevronDown, ChevronRight, Search } from 'lucide-react';

interface SwaggerSpec {
    info?: { title?: string; description?: string; version?: string };
    basePath?: string;
    paths?: Record<string, Record<string, SwaggerOperation>>;
}

interface SwaggerOperation {
    summary?: string;
    description?: string;
    tags?: string[];
    parameters?: SwaggerParameter[];
    responses?: Record<string, { description?: string }>;
    security?: Record<string, string[]>[];
}

interface SwaggerParameter {
    name: string;
    in: string;
    type?: string;
    description?: string;
    required?: boolean;
    schema?: { type?: string; properties?: Record<string, any> };
}

interface EndpointEntry {
    method: string;
    path: string;
    operation: SwaggerOperation;
    tag: string;
}

const METHOD_COLORS: Record<string, string> = {
    get: 'bg-green-100 text-green-700 border-green-300',
    post: 'bg-blue-100 text-blue-700 border-blue-300',
    put: 'bg-amber-100 text-amber-700 border-amber-300',
    patch: 'bg-orange-100 text-orange-700 border-orange-300',
    delete: 'bg-red-100 text-red-700 border-red-300',
};

const ApiReferencePage: React.FC = () => {
    const [spec, setSpec] = useState<SwaggerSpec | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [search, setSearch] = useState('');
    const [expanded, setExpanded] = useState<Set<string>>(new Set());

    useEffect(() => {
        // Try fetching from the API server first, fallback to bundled static copy
        fetch('/v1/swagger/doc.json')
            .then(r => {
                if (!r.ok) throw new Error('not found');
                return r.json();
            })
            .catch(() =>
                fetch('/swagger.json').then(r => {
                    if (!r.ok) throw new Error('fallback not found');
                    return r.json();
                })
            )
            .then(data => {
                setSpec(data);
                setLoading(false);
            })
            .catch(err => {
                setError('Failed to load API specification');
                setLoading(false);
                console.error('Swagger load error:', err);
            });
    }, []);

    const endpoints = useMemo((): EndpointEntry[] => {
        if (!spec?.paths) return [];
        const entries: EndpointEntry[] = [];
        for (const [path, methods] of Object.entries(spec.paths)) {
            for (const [method, operation] of Object.entries(methods)) {
                if (['get', 'post', 'put', 'patch', 'delete'].includes(method)) {
                    entries.push({
                        method,
                        path,
                        operation,
                        tag: operation.tags?.[0] || 'Other',
                    });
                }
            }
        }
        return entries;
    }, [spec]);

    const filtered = useMemo(() => {
        if (!search) return endpoints;
        const q = search.toLowerCase();
        return endpoints.filter(e =>
            e.path.toLowerCase().includes(q) ||
            e.method.toLowerCase().includes(q) ||
            e.operation.summary?.toLowerCase().includes(q) ||
            e.tag.toLowerCase().includes(q)
        );
    }, [endpoints, search]);

    const grouped = useMemo(() => {
        const groups: Record<string, EndpointEntry[]> = {};
        for (const e of filtered) {
            const tag = e.tag;
            if (!groups[tag]) groups[tag] = [];
            groups[tag].push(e);
        }
        return groups;
    }, [filtered]);

    const toggleExpand = (key: string) => {
        setExpanded(prev => {
            const next = new Set(prev);
            if (next.has(key)) next.delete(key);
            else next.add(key);
            return next;
        });
    };

    if (loading) {
        return (
            <div className="space-y-4">
                <Skeleton className="h-8 w-1/2" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-6 w-1/3 mt-4" />
                {Array.from({ length: 5 }).map((_, i) => (
                    <Skeleton key={i} className="h-12 w-full" />
                ))}
            </div>
        );
    }

    if (error) {
        return (
            <div className="text-center py-16 space-y-3">
                <h2 className="text-lg font-semibold">Failed to load API Reference</h2>
                <p className="text-sm text-muted-foreground">{error}</p>
            </div>
        );
    }

    return (
        <div className="space-y-6">
            {/* Header */}
            <div>
                <h1 className="text-2xl font-bold text-foreground">{spec?.info?.title || 'API Reference'}</h1>
                {spec?.info?.description && (
                    <p className="text-sm text-muted-foreground mt-1">{spec.info.description}</p>
                )}
                {spec?.info?.version && (
                    <Badge variant="outline" className="mt-2 text-xs">v{spec.info.version}</Badge>
                )}
            </div>

            {/* Search */}
            <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                    value={search}
                    onChange={e => setSearch(e.target.value)}
                    placeholder="Search endpoints by path, method, or tag..."
                    className="pl-10"
                />
            </div>

            {/* Stats */}
            <p className="text-xs text-muted-foreground">
                {filtered.length} endpoint{filtered.length !== 1 ? 's' : ''} across {Object.keys(grouped).length} group{Object.keys(grouped).length !== 1 ? 's' : ''}
            </p>

            {/* Grouped endpoints */}
            {Object.entries(grouped).map(([tag, entries]) => (
                <div key={tag} className="space-y-1">
                    <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground mb-2 mt-4">
                        {tag}
                    </h2>
                    {entries.map(e => {
                        const key = `${e.method}:${e.path}`;
                        const isOpen = expanded.has(key);

                        return (
                            <div key={key} className="border border-border rounded-lg overflow-hidden">
                                <button
                                    type="button"
                                    onClick={() => toggleExpand(key)}
                                    className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-muted/50 transition-colors"
                                >
                                    <Badge
                                        variant="outline"
                                        className={`uppercase text-[10px] font-bold min-w-[52px] justify-center ${METHOD_COLORS[e.method] || ''}`}
                                    >
                                        {e.method}
                                    </Badge>
                                    <code className="text-sm font-mono flex-1 text-foreground">{e.path}</code>
                                    <span className="text-xs text-muted-foreground hidden sm:inline truncate max-w-[200px]">
                                        {e.operation.summary}
                                    </span>
                                    {isOpen
                                        ? <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
                                        : <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />}
                                </button>

                                {isOpen && (
                                    <div className="px-4 pb-4 pt-2 border-t border-border bg-muted/30 space-y-4">
                                        {e.operation.summary && (
                                            <p className="text-sm font-medium text-foreground">{e.operation.summary}</p>
                                        )}
                                        {e.operation.description && (
                                            <p className="text-xs text-muted-foreground">{e.operation.description}</p>
                                        )}

                                        {/* Auth */}
                                        {e.operation.security && e.operation.security.length > 0 && (
                                            <div>
                                                <p className="text-xs font-medium text-muted-foreground mb-1">Authentication</p>
                                                <div className="flex gap-1">
                                                    {e.operation.security.map((s, i) => (
                                                        <Badge key={i} variant="outline" className="text-xs">
                                                            {Object.keys(s).join(', ')}
                                                        </Badge>
                                                    ))}
                                                </div>
                                            </div>
                                        )}

                                        {/* Parameters */}
                                        {e.operation.parameters && e.operation.parameters.length > 0 && (
                                            <div>
                                                <p className="text-xs font-medium text-muted-foreground mb-2">Parameters</p>
                                                <div className="overflow-x-auto">
                                                    <table className="w-full text-xs border-collapse">
                                                        <thead>
                                                            <tr className="border-b border-border">
                                                                <th className="text-left py-1.5 px-2 font-medium">Name</th>
                                                                <th className="text-left py-1.5 px-2 font-medium">In</th>
                                                                <th className="text-left py-1.5 px-2 font-medium">Type</th>
                                                                <th className="text-left py-1.5 px-2 font-medium">Required</th>
                                                                <th className="text-left py-1.5 px-2 font-medium">Description</th>
                                                            </tr>
                                                        </thead>
                                                        <tbody>
                                                            {e.operation.parameters.map((p, i) => (
                                                                <tr key={i} className="border-b border-border/50">
                                                                    <td className="py-1.5 px-2 font-mono">{p.name}</td>
                                                                    <td className="py-1.5 px-2">
                                                                        <Badge variant="outline" className="text-[10px]">{p.in}</Badge>
                                                                    </td>
                                                                    <td className="py-1.5 px-2 text-muted-foreground">{p.type || p.schema?.type || '—'}</td>
                                                                    <td className="py-1.5 px-2">
                                                                        {p.required ? <span className="text-red-500">Yes</span> : <span className="text-muted-foreground">No</span>}
                                                                    </td>
                                                                    <td className="py-1.5 px-2 text-muted-foreground">{p.description || '—'}</td>
                                                                </tr>
                                                            ))}
                                                        </tbody>
                                                    </table>
                                                </div>
                                            </div>
                                        )}

                                        {/* Responses */}
                                        {e.operation.responses && Object.keys(e.operation.responses).length > 0 && (
                                            <div>
                                                <p className="text-xs font-medium text-muted-foreground mb-2">Responses</p>
                                                <div className="flex flex-wrap gap-2">
                                                    {Object.entries(e.operation.responses).map(([code, resp]) => (
                                                        <div key={code} className="flex items-center gap-1.5">
                                                            <Badge
                                                                variant="outline"
                                                                className={`text-xs ${code.startsWith('2') ? 'bg-green-50 text-green-700' :
                                                                    code.startsWith('4') ? 'bg-amber-50 text-amber-700' :
                                                                        code.startsWith('5') ? 'bg-red-50 text-red-700' : ''}`}
                                                            >
                                                                {code}
                                                            </Badge>
                                                            <span className="text-xs text-muted-foreground">{resp.description}</span>
                                                        </div>
                                                    ))}
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                )}
                            </div>
                        );
                    })}
                </div>
            ))}

            {filtered.length === 0 && (
                <div className="text-center py-12">
                    <p className="text-sm text-muted-foreground">No endpoints match your search.</p>
                </div>
            )}
        </div>
    );
};

export default ApiReferencePage;
