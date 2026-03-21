import React, { useState } from 'react';
import { NavLink, Outlet, Link } from 'react-router-dom';
import { BookOpen, Rocket, FileText, Workflow, Tag, Radio, Layers, Code2, HelpCircle, ArrowLeft, Menu, Zap, Inbox, Globe } from 'lucide-react';
import { Sheet, SheetContent, SheetTrigger } from '../../components/ui/sheet';
import { Button } from '../../components/ui/button';

interface DocNavItem {
    label: string;
    to: string;
    icon: React.ReactNode;
}

interface DocNavSection {
    title: string;
    items: DocNavItem[];
}

const NAV_SECTIONS: DocNavSection[] = [
    {
        title: 'GUIDES',
        items: [
            { label: 'Getting Started', to: '/docs/getting-started', icon: <Rocket className="h-4 w-4" /> },
            { label: 'Templates', to: '/docs/templates', icon: <FileText className="h-4 w-4" /> },
            { label: 'Workflows', to: '/docs/workflows', icon: <Workflow className="h-4 w-4" /> },
            { label: 'Topics', to: '/docs/topics', icon: <Tag className="h-4 w-4" /> },
            { label: 'Channels', to: '/docs/channels', icon: <Radio className="h-4 w-4" /> },
            { label: 'SSE (Real-time)', to: '/docs/sse', icon: <Zap className="h-4 w-4" /> },
            { label: 'In-App (Inbox)', to: '/docs/in-app', icon: <Inbox className="h-4 w-4" /> },
            { label: 'Environments', to: '/docs/environments', icon: <Layers className="h-4 w-4" /> },
        ],
    },
    {
        title: 'EXAMPLES',
        items: [
            { label: 'Monkeys Integration', to: '/docs/monkeys-integration', icon: <Globe className="h-4 w-4" /> },
        ],
    },
    {
        title: 'REFERENCE',
        items: [
            { label: 'API Reference', to: '/docs/api', icon: <Code2 className="h-4 w-4" /> },
            { label: 'SDK Guide', to: '/docs/sdk', icon: <Code2 className="h-4 w-4" /> },
        ],
    },
    {
        title: 'HELP',
        items: [
            { label: 'Troubleshooting', to: '/docs/troubleshooting', icon: <HelpCircle className="h-4 w-4" /> },
        ],
    },
];

const linkClass = ({ isActive }: { isActive: boolean }) =>
    `flex items-center gap-2.5 px-3 py-2 text-sm rounded-md transition-colors ${isActive
        ? 'bg-accent/10 text-foreground font-medium'
        : 'text-muted-foreground hover:text-foreground hover:bg-muted'
    }`;

const SidebarContent: React.FC<{ onNavigate?: () => void }> = ({ onNavigate }) => (
    <div className="flex flex-col h-full">
        {/* Header */}
        <div className="px-4 py-5 border-b border-border">
            <div className="flex items-center gap-2.5">
                <BookOpen className="h-5 w-5 text-accent" />
                <span className="text-sm font-semibold tracking-tight">Documentation</span>
            </div>
        </div>

        {/* Back link */}
        <div className="px-3 pt-3">
            <Link
                to="/apps"
                onClick={onNavigate}
                className="flex items-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors px-3 py-1.5"
            >
                <ArrowLeft className="h-3 w-3" /> Back to Dashboard
            </Link>
        </div>

        {/* Navigation */}
        <nav className="flex-1 px-3 py-4 space-y-5 overflow-y-auto">
            {NAV_SECTIONS.map(section => (
                <div key={section.title}>
                    <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground px-3 mb-2">
                        {section.title}
                    </p>
                    <ul className="space-y-0.5">
                        {section.items.map(item => (
                            <li key={item.to}>
                                <NavLink to={item.to} className={linkClass} onClick={onNavigate}>
                                    {item.icon}
                                    {item.label}
                                </NavLink>
                            </li>
                        ))}
                    </ul>
                </div>
            ))}
        </nav>
    </div>
);

const DocsLayout: React.FC = () => {
    const [mobileOpen, setMobileOpen] = useState(false);

    return (
        <div className="flex h-full min-h-0">
            {/* Desktop sidebar */}
            <aside className="hidden md:flex w-56 flex-col border-r border-border bg-background shrink-0 overflow-y-auto">
                <SidebarContent />
            </aside>

            {/* Mobile sidebar */}
            <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
                <SheetTrigger asChild className="md:hidden fixed bottom-4 right-4 z-50">
                    <Button size="icon" variant="outline" className="rounded-full shadow-lg">
                        <Menu className="h-5 w-5" />
                    </Button>
                </SheetTrigger>
                <SheetContent side="left" className="w-56 p-0">
                    <SidebarContent onNavigate={() => setMobileOpen(false)} />
                </SheetContent>
            </Sheet>

            {/* Content area */}
            <div className="flex-1 overflow-y-auto">
                <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8">
                    <Outlet />
                </div>
            </div>
        </div>
    );
};

export default DocsLayout;
