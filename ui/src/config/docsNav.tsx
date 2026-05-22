import type React from 'react';
import {
    BookOpen,
    Rocket,
    FileText,
    Workflow,
    Tag,
    Radio,
    Code2,
    HelpCircle,
    Zap,
    Inbox,
    Layers,
    Globe,
    IndianRupee,
} from 'lucide-react';

export interface DocNavItem {
    label: string;
    to: string;
    icon: React.ReactNode;
}

export interface DocNavSection {
    title: string;
    items: DocNavItem[];
}

export const NAV_SECTIONS: DocNavSection[] = [
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
        title: 'BILLING',
        items: [
            { label: 'Pricing & Credits', to: '/docs/pricing', icon: <IndianRupee className="h-4 w-4" /> },
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

export const DOCS_HEADER_ICON = <BookOpen className="h-5 w-5 text-accent" />;

/** Flat list for breadcrumb label lookup */
export const ALL_DOC_NAV_ITEMS: DocNavItem[] = NAV_SECTIONS.flatMap((s) => s.items);

export function getDocLabelForPath(pathname: string): string {
    if (pathname === '/docs/api') {
        return 'API Reference';
    }
    const item = ALL_DOC_NAV_ITEMS.find((i) => i.to === pathname);
    if (item) {
        return item.label;
    }
    const slug = pathname.replace(/^\/docs\//, '');
    if (!slug || slug === 'docs') {
        return 'Documentation';
    }
    return slug
        .split('-')
        .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
        .join(' ');
}
