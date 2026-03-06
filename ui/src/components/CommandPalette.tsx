import React, { useEffect, useState, useMemo, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Dialog, DialogContent } from './ui/dialog';
import { Input } from './ui/input';
import { applicationsAPI } from '../services/api';
import { useAuth } from '../contexts/AuthContext';
import {
    Search, LayoutGrid, Workflow, Timer, Tag, BarChart3, ScrollText,
    BookOpen, Plus, KeyRound, ArrowRight,
} from 'lucide-react';

interface CommandItem {
    id: string;
    label: string;
    icon: React.ReactNode;
    section: string;
    action: () => void;
    keywords?: string;
}

const CommandPalette: React.FC = () => {
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const [activeIndex, setActiveIndex] = useState(0);
    const [apps, setApps] = useState<{ app_id: string; app_name: string }[]>([]);
    const navigate = useNavigate();
    const { isAuthenticated } = useAuth();
    const inputRef = useRef<HTMLInputElement>(null);
    const listRef = useRef<HTMLDivElement>(null);

    // Global keyboard shortcut
    useEffect(() => {
        const handler = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
                e.preventDefault();
                if (isAuthenticated) setOpen(prev => !prev);
            }
        };
        document.addEventListener('keydown', handler);
        return () => document.removeEventListener('keydown', handler);
    }, [isAuthenticated]);

    // Load apps when palette opens
    useEffect(() => {
        if (!open) return;
        setQuery('');
        setActiveIndex(0);
        applicationsAPI.list()
            .then(resp => {
                const list = Array.isArray(resp) ? resp : (resp as any)?.applications || [];
                setApps(list.map((a: any) => ({ app_id: a.app_id, app_name: a.app_name || a.name })));
            })
            .catch(() => setApps([]));
    }, [open]);

    const go = useCallback((path: string) => {
        setOpen(false);
        navigate(path);
    }, [navigate]);

    const items = useMemo((): CommandItem[] => {
        const navItems: CommandItem[] = [
            { id: 'nav-apps', label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, section: 'Navigation', action: () => go('/apps'), keywords: 'apps list' },
            { id: 'nav-workflows', label: 'Workflows', icon: <Workflow className="h-4 w-4" />, section: 'Navigation', action: () => go('/workflows'), keywords: 'automation pipelines' },
            { id: 'nav-digest', label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, section: 'Navigation', action: () => go('/digest-rules'), keywords: 'batching aggregation' },
            { id: 'nav-topics', label: 'Topics', icon: <Tag className="h-4 w-4" />, section: 'Navigation', action: () => go('/topics'), keywords: 'subscriptions broadcast' },
            { id: 'nav-dashboard', label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, section: 'Navigation', action: () => go('/dashboard'), keywords: 'analytics overview stats' },
            { id: 'nav-audit', label: 'Audit Logs', icon: <ScrollText className="h-4 w-4" />, section: 'Navigation', action: () => go('/audit'), keywords: 'history events' },
            { id: 'nav-docs', label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, section: 'Navigation', action: () => go('/docs'), keywords: 'help guides reference' },
        ];

        const actionItems: CommandItem[] = [
            { id: 'act-create-app', label: 'Create Application', icon: <Plus className="h-4 w-4" />, section: 'Actions', action: () => go('/apps'), keywords: 'new application' },
            { id: 'act-create-workflow', label: 'Create Workflow', icon: <Plus className="h-4 w-4" />, section: 'Actions', action: () => go('/workflows/new'), keywords: 'new workflow' },
            { id: 'act-api-ref', label: 'API Reference', icon: <KeyRound className="h-4 w-4" />, section: 'Actions', action: () => go('/docs/api'), keywords: 'swagger openapi endpoints' },
        ];

        const appItems: CommandItem[] = apps.map(a => ({
            id: `app-${a.app_id}`,
            label: a.app_name,
            icon: <ArrowRight className="h-4 w-4" />,
            section: 'Applications',
            action: () => go(`/apps/${a.app_id}`),
            keywords: `app ${a.app_name}`,
        }));

        return [...navItems, ...actionItems, ...appItems];
    }, [apps, go]);

    const filtered = useMemo(() => {
        if (!query) return items;
        const q = query.toLowerCase();
        return items.filter(item =>
            item.label.toLowerCase().includes(q) ||
            item.section.toLowerCase().includes(q) ||
            item.keywords?.toLowerCase().includes(q)
        );
    }, [items, query]);

    const grouped = useMemo(() => {
        const groups: Record<string, CommandItem[]> = {};
        for (const item of filtered) {
            if (!groups[item.section]) groups[item.section] = [];
            groups[item.section].push(item);
        }
        return groups;
    }, [filtered]);

    // Reset active index when results change
    useEffect(() => {
        setActiveIndex(0);
    }, [filtered.length]);

    // Scroll active item into view
    useEffect(() => {
        const el = listRef.current?.querySelector(`[data-index="${activeIndex}"]`);
        el?.scrollIntoView({ block: 'nearest' });
    }, [activeIndex]);

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === 'ArrowDown') {
            e.preventDefault();
            setActiveIndex(i => Math.min(i + 1, filtered.length - 1));
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            setActiveIndex(i => Math.max(i - 1, 0));
        } else if (e.key === 'Enter' && filtered[activeIndex]) {
            e.preventDefault();
            filtered[activeIndex].action();
        }
    };

    if (!isAuthenticated) return null;

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent className="p-0 gap-0 max-w-lg overflow-hidden [&>button]:hidden">
                {/* Search input */}
                <div className="flex items-center gap-2 border-b border-border px-4 py-3">
                    <Search className="h-4 w-4 text-muted-foreground shrink-0" />
                    <Input
                        ref={inputRef}
                        value={query}
                        onChange={e => setQuery(e.target.value)}
                        onKeyDown={handleKeyDown}
                        placeholder="Search or type a command..."
                        className="border-0 shadow-none focus-visible:ring-0 text-sm h-auto p-0"
                        autoFocus
                    />
                    <kbd className="hidden sm:inline text-[10px] text-muted-foreground bg-muted px-1.5 py-0.5 rounded border font-mono">
                        ESC
                    </kbd>
                </div>

                {/* Results */}
                <div ref={listRef} className="max-h-72 overflow-y-auto p-2">
                    {filtered.length === 0 ? (
                        <p className="text-sm text-muted-foreground text-center py-6">No results found.</p>
                    ) : (
                        Object.entries(grouped).map(([section, sectionItems]) => (
                            <div key={section} className="mb-2">
                                <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground px-2 py-1">
                                    {section}
                                </p>
                                {sectionItems.map(item => {
                                    const idx = filtered.indexOf(item);
                                    const isActive = idx === activeIndex;
                                    return (
                                        <button
                                            key={item.id}
                                            type="button"
                                            data-index={idx}
                                            onClick={() => item.action()}
                                            onMouseEnter={() => setActiveIndex(idx)}
                                            className={`w-full flex items-center gap-3 px-3 py-2 text-sm rounded-md transition-colors ${isActive ? 'bg-muted text-foreground' : 'text-muted-foreground hover:bg-muted'
                                                }`}
                                        >
                                            {item.icon}
                                            <span className="flex-1 text-left truncate">{item.label}</span>
                                            {isActive && (
                                                <kbd className="text-[10px] bg-background px-1.5 py-0.5 rounded border font-mono text-muted-foreground">
                                                    ↵
                                                </kbd>
                                            )}
                                        </button>
                                    );
                                })}
                            </div>
                        ))
                    )}
                </div>

                {/* Footer */}
                <div className="border-t border-border px-4 py-2 flex items-center justify-between text-[10px] text-muted-foreground">
                    <span>
                        <kbd className="bg-muted px-1 py-0.5 rounded border font-mono mr-1">↑↓</kbd>
                        navigate
                        <kbd className="bg-muted px-1 py-0.5 rounded border font-mono mx-1">↵</kbd>
                        select
                    </span>
                    <span>
                        <kbd className="bg-muted px-1 py-0.5 rounded border font-mono mr-1">Ctrl+K</kbd>
                        toggle
                    </span>
                </div>
            </DialogContent>
        </Dialog>
    );
};

export default CommandPalette;
