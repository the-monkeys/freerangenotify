import type React from 'react';
import {
    LayoutDashboard,
    Users,
    FileText,
    Bell,
    Layers,
    MessageSquare,
    UsersRound,
    Plug,
    GitBranch,
    Settings,
    Code,
    Workflow,
    Timer,
    Link2,
    MessageCircle,
} from 'lucide-react';

export type TabId =
    | 'overview'
    | 'users'
    | 'templates'
    | 'notifications'
    | 'digest-rules'
    | 'workflows'
    | 'schedules'
    | 'topics'
    | 'team'
    | 'providers'
    | 'environments'
    | 'settings'
    | 'integration'
    | 'import'
    | 'browse-library'
    | 'whatsapp';

export const VALID_TABS: TabId[] = [
    'overview',
    'users',
    'templates',
    'notifications',
    'digest-rules',
    'workflows',
    'schedules',
    'topics',
    'team',
    'providers',
    'environments',
    'settings',
    'integration',
    'import',
    'browse-library',
    'whatsapp',
];

export interface TabDef {
    id: TabId;
    label: string;
    icon: React.ReactNode;
}

export interface TabGroup {
    label: string;
    tabs: TabDef[];
}

export const TAB_GROUPS: TabGroup[] = [
    {
        label: 'General',
        tabs: [
            { id: 'overview', label: 'Overview', icon: <LayoutDashboard className="h-4 w-4" /> },
            { id: 'users', label: 'Users', icon: <Users className="h-4 w-4" /> },
            { id: 'templates', label: 'Templates', icon: <FileText className="h-4 w-4" /> },
            { id: 'notifications', label: 'Notifications', icon: <Bell className="h-4 w-4" /> },
        ],
    },
    {
        label: 'Configuration',
        tabs: [
            { id: 'digest-rules', label: 'Digest Rules', icon: <Layers className="h-4 w-4" /> },
            { id: 'workflows', label: 'Workflows', icon: <Workflow className="h-4 w-4" /> },
            { id: 'schedules', label: 'Schedules', icon: <Timer className="h-4 w-4" /> },
            { id: 'topics', label: 'Topics', icon: <MessageSquare className="h-4 w-4" /> },
            { id: 'whatsapp', label: 'WhatsApp', icon: <MessageCircle className="h-4 w-4" /> },
            { id: 'team', label: 'Team', icon: <UsersRound className="h-4 w-4" /> },
            { id: 'providers', label: 'Providers', icon: <Plug className="h-4 w-4" /> },
            { id: 'import', label: 'Import', icon: <Link2 className="h-4 w-4" /> },
        ],
    },
    {
        label: 'Advanced',
        tabs: [
            { id: 'environments', label: 'Environments', icon: <GitBranch className="h-4 w-4" /> },
            { id: 'settings', label: 'Settings', icon: <Settings className="h-4 w-4" /> },
            { id: 'integration', label: 'Integration', icon: <Code className="h-4 w-4" /> },
        ],
    },
];

export const ALL_TABS: TabDef[] = TAB_GROUPS.flatMap((g) => g.tabs);

export function isTabActive(
    tabId: TabId,
    activeTab: TabId,
    pathname: string,
): boolean {
    if (activeTab === tabId) {
        return true;
    }
    if (tabId === 'templates') {
        return (
            activeTab === 'browse-library' ||
            pathname.includes('/templates/library')
        );
    }
    return false;
}
