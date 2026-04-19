import React, { useMemo } from 'react';
import { Check, CheckCheck } from 'lucide-react';
import type { TwilioContentTemplate } from '../../types';

interface WhatsAppPreviewProps {
    /** The Twilio content template to preview (optional — falls back to raw body) */
    template?: TwilioContentTemplate;
    /** Raw body text; used when `template` is not provided */
    body?: string;
    /** Variable substitutions: { "1": "Dave", "2": "ORDER-123" } */
    variables?: Record<string, string>;
    /** Optional header text (e.g. friendly_name) shown above the bubble */
    header?: string;
    /** Render compact (smaller phone chrome) */
    compact?: boolean;
}

function resolveBody(tpl?: TwilioContentTemplate, fallback?: string): {
    body: string;
    mediaUrl?: string;
    actions?: string[];
} {
    if (!tpl?.types) return { body: fallback || '' };

    // Preference order matches Twilio content type precedence
    const order = [
        'twilio/card',
        'twilio/list-picker',
        'twilio/quick-reply',
        'twilio/call-to-action',
        'twilio/media',
        'twilio/text',
    ];

    for (const key of order) {
        const t = tpl.types[key];
        if (!t) continue;
        const body: string = t.body || t.title || '';
        if (!body && !t.media) continue;

        const actions: string[] = [];
        if (Array.isArray(t.actions)) {
            for (const a of t.actions) {
                if (a?.title) actions.push(a.title);
                else if (a?.text) actions.push(a.text);
            }
        }
        let mediaUrl: string | undefined;
        if (Array.isArray(t.media) && t.media.length > 0) mediaUrl = t.media[0];

        return { body, mediaUrl, actions };
    }

    return { body: fallback || '' };
}

function substituteVariables(text: string, vars: Record<string, string>): React.ReactNode {
    if (!text) return '';
    // Match {{1}}, {{name}}, etc.
    const parts: React.ReactNode[] = [];
    const regex = /\{\{\s*([^}\s]+)\s*\}\}/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    let idx = 0;
    while ((match = regex.exec(text)) !== null) {
        if (match.index > lastIndex) {
            parts.push(text.slice(lastIndex, match.index));
        }
        const key = match[1];
        const value = vars[key];
        if (value && value.trim() !== '') {
            parts.push(<strong key={`v-${idx}`}>{value}</strong>);
        } else {
            parts.push(
                <span
                    key={`v-${idx}`}
                    className="rounded bg-yellow-200/70 dark:bg-yellow-400/30 px-1 font-mono text-[0.85em]"
                    title={`Variable {{${key}}} not set`}
                >
                    {`{{${key}}}`}
                </span>,
            );
        }
        idx++;
        lastIndex = match.index + match[0].length;
    }
    if (lastIndex < text.length) parts.push(text.slice(lastIndex));
    return parts;
}

const WhatsAppPreview: React.FC<WhatsAppPreviewProps> = ({
    template,
    body,
    variables,
    header,
    compact = false,
}) => {
    const resolved = useMemo(() => resolveBody(template, body), [template, body]);
    const vars = variables || {};
    const now = useMemo(
        () => new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        [],
    );

    const wrapperWidth = compact ? 'max-w-[260px]' : 'max-w-[320px]';

    return (
        <div className={`flex flex-col items-center ${wrapperWidth} w-full`}>
            {header && (
                <div className="text-xs text-muted-foreground mb-2 w-full text-center truncate">
                    {header}
                </div>
            )}
            {/* Phone chrome */}
            <div className="w-full rounded-[2rem] border border-border bg-[#0a140c] p-2 shadow-xl">
                {/* Status bar */}
                <div className="flex items-center justify-between px-4 pt-1 pb-1 text-[10px] text-white/70">
                    <span>9:41</span>
                    <span>••• WhatsApp</span>
                </div>
                {/* Header */}
                <div className="flex items-center gap-2 rounded-t-xl bg-[#075E54] px-3 py-2">
                    <div className="h-7 w-7 rounded-full bg-white/20 flex items-center justify-center text-white text-xs font-semibold">
                        B
                    </div>
                    <div className="flex flex-col">
                        <span className="text-white text-xs font-semibold">Business</span>
                        <span className="text-white/70 text-[10px]">online</span>
                    </div>
                </div>
                {/* Chat background */}
                <div
                    className="rounded-b-xl px-3 py-4 min-h-[200px]"
                    style={{
                        backgroundColor: '#ECE5DD',
                        backgroundImage:
                            'radial-gradient(circle at 20% 20%, rgba(0,0,0,0.03) 1px, transparent 1px), radial-gradient(circle at 80% 60%, rgba(0,0,0,0.03) 1px, transparent 1px)',
                        backgroundSize: '16px 16px',
                    }}
                >
                    {/* Message bubble (incoming — from business) */}
                    <div className="flex justify-start">
                        <div className="relative max-w-[85%] rounded-lg rounded-tl-none bg-white px-3 py-2 shadow-sm">
                            {resolved.mediaUrl && (
                                <div className="mb-2 overflow-hidden rounded">
                                    {/* Simple image render; fallback to filename if not loadable */}
                                    {/* eslint-disable-next-line jsx-a11y/alt-text */}
                                    <img
                                        src={resolved.mediaUrl}
                                        className="max-h-40 w-full object-cover"
                                        onError={(e) => {
                                            (e.target as HTMLImageElement).style.display = 'none';
                                        }}
                                    />
                                </div>
                            )}
                            <div className="text-[13px] leading-snug text-gray-900 whitespace-pre-wrap break-words">
                                {substituteVariables(resolved.body, vars)}
                            </div>
                            <div className="mt-1 flex items-center justify-end gap-1 text-[10px] text-gray-500">
                                <span>{now}</span>
                                <CheckCheck className="h-3 w-3 text-[#34B7F1]" />
                            </div>
                        </div>
                    </div>

                    {/* Quick-reply / CTA buttons */}
                    {resolved.actions && resolved.actions.length > 0 && (
                        <div className="mt-2 flex flex-col gap-1">
                            {resolved.actions.map((a, i) => (
                                <div
                                    key={i}
                                    className="w-full rounded-md bg-white text-center text-[12px] font-medium text-[#00a5f4] py-1.5 shadow-sm"
                                >
                                    {a}
                                </div>
                            ))}
                        </div>
                    )}
                </div>
                {/* Composer */}
                <div className="mt-2 flex items-center gap-2 px-2 pb-1">
                    <div className="flex-1 rounded-full bg-white/10 px-3 py-1 text-[10px] text-white/50">
                        Message
                    </div>
                    <Check className="h-3 w-3 text-white/50" />
                </div>
            </div>
        </div>
    );
};

export default WhatsAppPreview;
