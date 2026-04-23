import React from 'react';
import type { ProviderKind } from '../../../types';

interface ProviderTypePickerProps {
    onSelect: (kind: ProviderKind | 'custom') => void;
}

const PROVIDER_TYPES: { kind: ProviderKind | 'custom'; label: string; description: string; icon: string }[] = [
    { kind: 'discord', label: 'Discord', description: 'Incoming webhook with embeds', icon: '🎮' },
    { kind: 'slack', label: 'Slack', description: 'Incoming webhook with Block Kit', icon: '💬' },
    { kind: 'teams', label: 'Microsoft Teams', description: 'Workflow or legacy connector', icon: '🟦' },
    { kind: 'generic', label: 'Generic Webhook', description: 'Any HTTP endpoint with HMAC signing', icon: '🔗' },
    { kind: 'custom', label: 'Custom', description: 'Full control over channel, headers, and URL', icon: '⚙️' },
];

const ProviderTypePicker: React.FC<ProviderTypePickerProps> = ({ onSelect }) => {
    return (
        <div className="space-y-3">
            <p className="text-sm text-muted-foreground">Choose the type of provider to register:</p>
            <div
                className="grid gap-3"
                style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))' }}
                role="radiogroup"
                aria-label="Provider type"
            >
                {PROVIDER_TYPES.map(pt => (
                    <button
                        key={pt.kind}
                        type="button"
                        role="radio"
                        aria-checked={false}
                        onClick={() => onSelect(pt.kind)}
                        className="flex flex-col items-center gap-1.5 rounded-lg border border-border/70 p-4 text-center
                                   hover:border-primary hover:bg-accent/40 transition-colors cursor-pointer
                                   focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    >
                        <span className="text-2xl">{pt.icon}</span>
                        <span className="text-sm font-medium">{pt.label}</span>
                        <span className="text-[11px] text-muted-foreground leading-tight">{pt.description}</span>
                    </button>
                ))}
            </div>
        </div>
    );
};

export default ProviderTypePicker;
