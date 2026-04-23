import React from 'react';
import { Badge } from '../ui/badge';

// ── Types matching backend notification.Content rich fields ──

export interface ContentAttachment {
    type: string; // image | video | file | audio
    url: string;
    name?: string;
    mime_type?: string;
    alt_text?: string;
}

export interface ContentAction {
    type: string; // link | submit | dismiss
    label: string;
    url?: string;
    value?: string;
    style?: string; // primary | danger | default
}

export interface ContentField {
    key: string;
    value: string;
    inline?: boolean;
}

export interface ContentMention {
    platform: string;
    platform_id: string;
    display?: string;
}

export interface ContentPollChoice {
    label: string;
    emoji?: string;
}

export interface ContentPoll {
    question: string;
    choices: ContentPollChoice[];
    multi_select?: boolean;
    duration_hours?: number;
}

export interface ContentStyle {
    severity?: 'info' | 'success' | 'warning' | 'danger';
    color?: string;
}

export interface RichContent {
    title?: string;
    body?: string;
    data?: Record<string, any>;
    media_url?: string;
    attachments?: ContentAttachment[];
    actions?: ContentAction[];
    fields?: ContentField[];
    mentions?: ContentMention[];
    poll?: ContentPoll;
    style?: ContentStyle;
}

export interface ChannelPreviewProps {
    channel: string;
    payloadKind?: string;
    content: RichContent;
}

// ── Severity → color mappings ──

const SEVERITY_COLORS: Record<string, { discord: number; hex: string; label: string }> = {
    info: { discord: 3447003, hex: '#3498DB', label: 'Info' },
    success: { discord: 3066993, hex: '#2ECC71', label: 'Success' },
    warning: { discord: 15105570, hex: '#E67E22', label: 'Warning' },
    danger: { discord: 15158332, hex: '#E74C3C', label: 'Danger' },
};

// ── Resolve effective preview kind ──

function resolveKind(channel: string, payloadKind?: string): string {
    if (payloadKind && payloadKind !== 'generic') return payloadKind;
    if (['discord', 'slack', 'teams'].includes(channel)) return channel;
    return 'generic';
}

// ── Discord Preview ──

const DiscordPreview: React.FC<{ content: RichContent }> = ({ content }) => {
    const severity = content.style?.severity;
    const color = severity ? SEVERITY_COLORS[severity]?.hex ?? '#3498DB' : (content.style?.color || '#3498DB');

    return (
        <div className="rounded-lg bg-[#313338] p-4 text-sm text-[#DBDEE1] font-sans">
            <div className="flex gap-3">
                <div className="w-1 rounded-full flex-shrink-0" style={{ backgroundColor: color }} />
                <div className="min-w-0 flex-1 space-y-2">
                    {content.title && (
                        <p className="font-semibold text-white">{content.title}</p>
                    )}
                    {content.body && (
                        <p className="text-[#B5BAC1] whitespace-pre-wrap">{content.body}</p>
                    )}

                    {/* Fields */}
                    {content.fields && content.fields.length > 0 && (
                        <div className="grid grid-cols-3 gap-2 mt-2">
                            {content.fields.map((f, i) => (
                                <div key={i} className={f.inline ? 'col-span-1' : 'col-span-3'}>
                                    <p className="text-xs font-semibold text-[#B5BAC1]">{f.key}</p>
                                    <p className="text-sm text-[#DBDEE1]">{f.value}</p>
                                </div>
                            ))}
                        </div>
                    )}

                    {/* Image attachments */}
                    {content.attachments?.filter(a => a.type === 'image').map((a, i) => (
                        <div key={i} className="mt-2">
                            <img
                                src={a.url}
                                alt={a.alt_text || a.name || 'attachment'}
                                className="max-w-[300px] rounded"
                                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                            />
                        </div>
                    ))}

                    {/* Actions as embed links */}
                    {content.actions && content.actions.length > 0 && (
                        <div className="flex gap-2 mt-2 flex-wrap">
                            {content.actions.map((a, i) => (
                                <span key={i} className="text-[#00A8FC] text-xs underline cursor-pointer">
                                    {a.label}
                                </span>
                            ))}
                        </div>
                    )}

                    {/* Poll */}
                    {content.poll && (
                        <div className="mt-3 space-y-1.5">
                            <p className="text-xs font-semibold text-white">{content.poll.question}</p>
                            {content.poll.choices.map((c, i) => (
                                <div key={i} className="flex items-center gap-2 rounded bg-[#2B2D31] px-3 py-1.5 text-xs">
                                    {c.emoji && <span>{c.emoji}</span>}
                                    <span>{c.label}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

// ── Slack Preview ──

const SlackPreview: React.FC<{ content: RichContent }> = ({ content }) => {
    const severity = content.style?.severity;
    const accentColor = severity ? SEVERITY_COLORS[severity]?.hex ?? '#4A154B' : (content.style?.color || '#4A154B');

    return (
        <div className="rounded-lg bg-white border border-[#DDDDDD] p-4 text-sm font-sans">
            <div className="flex gap-3">
                <div className="w-1 rounded-full flex-shrink-0" style={{ backgroundColor: accentColor }} />
                <div className="min-w-0 flex-1 space-y-2">
                    {content.title && (
                        <p className="font-bold text-[#1D1C1D]">{content.title}</p>
                    )}
                    {content.body && (
                        <p className="text-[#1D1C1D] whitespace-pre-wrap">{content.body}</p>
                    )}

                    {/* Fields */}
                    {content.fields && content.fields.length > 0 && (
                        <div className="grid grid-cols-2 gap-x-4 gap-y-1 mt-2">
                            {content.fields.map((f, i) => (
                                <div key={i} className={f.inline ? 'col-span-1' : 'col-span-2'}>
                                    <p className="text-xs font-bold text-[#616061]">{f.key}</p>
                                    <p className="text-sm text-[#1D1C1D]">{f.value}</p>
                                </div>
                            ))}
                        </div>
                    )}

                    {/* Image attachments */}
                    {content.attachments?.filter(a => a.type === 'image').map((a, i) => (
                        <div key={i} className="mt-2">
                            <img
                                src={a.url}
                                alt={a.alt_text || a.name || 'image'}
                                className="max-w-[360px] rounded"
                                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                            />
                        </div>
                    ))}

                    {/* Actions as buttons */}
                    {content.actions && content.actions.length > 0 && (
                        <div className="flex gap-2 mt-2 flex-wrap">
                            {content.actions.map((a, i) => (
                                <button
                                    key={i}
                                    className={`rounded px-3 py-1 text-xs font-medium border ${a.style === 'primary'
                                            ? 'bg-[#007a5a] text-white border-[#007a5a]'
                                            : a.style === 'danger'
                                                ? 'bg-[#E01E5A] text-white border-[#E01E5A]'
                                                : 'bg-white text-[#1D1C1D] border-[#DDDDDD]'
                                        }`}
                                >
                                    {a.label}
                                </button>
                            ))}
                        </div>
                    )}

                    {/* Poll as radio buttons */}
                    {content.poll && (
                        <div className="mt-3 space-y-1.5 border-t border-[#DDDDDD] pt-2">
                            <p className="text-xs font-bold text-[#1D1C1D]">{content.poll.question}</p>
                            {content.poll.choices.map((ch, i) => (
                                <label key={i} className="flex items-center gap-2 text-xs text-[#1D1C1D]">
                                    <input type={content.poll?.multi_select ? 'checkbox' : 'radio'} name="poll" disabled className="accent-[#4A154B]" />
                                    {ch.emoji && <span>{ch.emoji}</span>}
                                    {ch.label}
                                </label>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

// ── Teams Preview (Adaptive Card style) ──

const TeamsPreview: React.FC<{ content: RichContent }> = ({ content }) => {
    const severity = content.style?.severity;
    const accentColor = severity ? SEVERITY_COLORS[severity]?.hex ?? '#6264A7' : (content.style?.color || '#6264A7');

    return (
        <div className="rounded-lg bg-white border border-[#E1DFDD] p-4 text-sm font-sans shadow-sm">
            <div className="border-l-4 pl-3 space-y-2" style={{ borderColor: accentColor }}>
                {content.title && (
                    <p className="font-semibold text-[#252423]">{content.title}</p>
                )}
                {content.body && (
                    <p className="text-[#605E5C] whitespace-pre-wrap">{content.body}</p>
                )}

                {/* FactSet */}
                {content.fields && content.fields.length > 0 && (
                    <div className="mt-2 space-y-1">
                        {content.fields.map((f, i) => (
                            <div key={i} className="flex gap-2">
                                <span className="text-xs font-semibold text-[#605E5C] min-w-[100px]">{f.key}:</span>
                                <span className="text-xs text-[#252423]">{f.value}</span>
                            </div>
                        ))}
                    </div>
                )}

                {/* Images */}
                {content.attachments?.filter(a => a.type === 'image').map((a, i) => (
                    <div key={i} className="mt-2">
                        <img
                            src={a.url}
                            alt={a.alt_text || a.name || 'image'}
                            className="max-w-[300px] rounded"
                            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                        />
                    </div>
                ))}

                {/* ActionSet */}
                {content.actions && content.actions.length > 0 && (
                    <div className="flex gap-2 mt-3 flex-wrap">
                        {content.actions.map((a, i) => (
                            <button
                                key={i}
                                className={`rounded px-3 py-1.5 text-xs font-medium ${a.style === 'primary'
                                        ? 'bg-[#6264A7] text-white'
                                        : a.style === 'danger'
                                            ? 'bg-[#C4314B] text-white'
                                            : 'bg-[#F3F2F1] text-[#252423]'
                                    }`}
                            >
                                {a.label}
                            </button>
                        ))}
                    </div>
                )}

                {/* Input.ChoiceSet for polls */}
                {content.poll && (
                    <div className="mt-3 space-y-1.5 border-t border-[#E1DFDD] pt-2">
                        <p className="text-xs font-semibold text-[#252423]">{content.poll.question}</p>
                        {content.poll.choices.map((ch, i) => (
                            <label key={i} className="flex items-center gap-2 text-xs text-[#252423]">
                                <input type={content.poll?.multi_select ? 'checkbox' : 'radio'} name="poll" disabled className="accent-[#6264A7]" />
                                {ch.emoji && <span>{ch.emoji}</span>}
                                {ch.label}
                            </label>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
};

// ── Generic Preview (JSON pretty-print) ──

const GenericPreview: React.FC<{ content: RichContent }> = ({ content }) => {
    return (
        <div className="rounded-lg bg-zinc-900 p-4 text-sm font-mono">
            <pre className="text-green-400 whitespace-pre-wrap overflow-auto max-h-[400px]">
                {JSON.stringify(content, null, 2)}
            </pre>
        </div>
    );
};

// ── Facade ──

const ChannelPreview: React.FC<ChannelPreviewProps> = ({ channel, payloadKind, content }) => {
    const kind = resolveKind(channel, payloadKind);

    const hasRich = (content.attachments?.length ?? 0) > 0
        || (content.actions?.length ?? 0) > 0
        || (content.fields?.length ?? 0) > 0
        || content.poll != null
        || content.style != null
        || (content.mentions?.length ?? 0) > 0;

    return (
        <div className="space-y-2">
            <div className="flex items-center gap-2">
                <Badge variant="secondary" className="text-[10px] uppercase">{kind} preview</Badge>
                {hasRich && <Badge variant="outline" className="text-[10px]">Rich content</Badge>}
            </div>
            {kind === 'discord' && <DiscordPreview content={content} />}
            {kind === 'slack' && <SlackPreview content={content} />}
            {kind === 'teams' && <TeamsPreview content={content} />}
            {kind === 'generic' && <GenericPreview content={content} />}
        </div>
    );
};

export default ChannelPreview;
export { DiscordPreview, SlackPreview, TeamsPreview, GenericPreview };
