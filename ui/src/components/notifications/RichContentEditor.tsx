import React from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/tabs';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Checkbox } from '../ui/checkbox';
import { Badge } from '../ui/badge';
import { Plus, X } from 'lucide-react';
import type {
    ContentAttachment,
    ContentAction,
    ContentField,
    ContentMention,
    ContentPoll,
    ContentStyle,
} from '../channels/ChannelPreview';

export interface RichContentData {
    attachments: ContentAttachment[];
    actions: ContentAction[];
    fields: ContentField[];
    mentions: ContentMention[];
    poll: ContentPoll | null;
    style: ContentStyle | null;
}

interface RichContentEditorProps {
    value: RichContentData;
    onChange: (data: RichContentData) => void;
    /** Optional: show a JSON preview of the payload fields sent to the API. Defaults to true. */
    showJsonPreview?: boolean;
}

const EMPTY_RICH: RichContentData = {
    attachments: [],
    actions: [],
    fields: [],
    mentions: [],
    poll: null,
    style: null,
};

export function emptyRichContent(): RichContentData {
    return { ...EMPTY_RICH, attachments: [], actions: [], fields: [], mentions: [] };
}

export function isRichContentEmpty(data: RichContentData): boolean {
    return data.attachments.length === 0
        && data.actions.length === 0
        && data.fields.length === 0
        && data.mentions.length === 0
        && !data.poll
        && !data.style;
}

/**
 * Merge rich content data into a notification payload's content object.
 *
 * Filters out incomplete entries the user may have left blank in the UI:
 *   - actions without label or (for `link`) url
 *   - fields without key or value
 *   - attachments without url
 *   - mentions without platform_id
 *   - poll choices without label (and the whole poll if it ends up choice-less)
 *
 * The backend rejects any of these as `invalid notification: content.<field>: ...`
 * so dropping them client-side keeps the request from 500-ing.
 */
export function richContentToPayload(data: RichContentData): Record<string, any> {
    const out: Record<string, any> = {};

    const attachments = data.attachments.filter((a) => a && a.url && a.url.trim() !== '');
    if (attachments.length > 0) out.attachments = attachments;

    const actions = data.actions.filter((a) => {
        if (!a || !a.label || a.label.trim() === '') return false;
        if (a.type === 'link' && (!a.url || a.url.trim() === '')) return false;
        return true;
    });
    if (actions.length > 0) out.actions = actions;

    const fields = data.fields.filter((f) => f && f.key && f.key.trim() !== '' && f.value !== undefined && String(f.value).trim() !== '');
    if (fields.length > 0) out.fields = fields;

    const mentions = data.mentions.filter((m) => m && m.platform_id && m.platform_id.trim() !== '');
    if (mentions.length > 0) out.mentions = mentions;

    if (data.poll) {
        const choices = data.poll.choices.filter((c) => c && c.label && c.label.trim() !== '');
        if (data.poll.question && data.poll.question.trim() !== '' && choices.length >= 2) {
            out.poll = { ...data.poll, choices };
        }
    }

    if (data.style && (data.style.severity || data.style.color)) {
        out.style = data.style;
    }

    return out;
}

const RichContentEditor: React.FC<RichContentEditorProps> = ({ value, onChange, showJsonPreview = true }) => {
    const update = (patch: Partial<RichContentData>) => onChange({ ...value, ...patch });
    const payloadPreview = richContentToPayload(value);

    return (
        <div className="rounded-lg border border-border/70 bg-muted/25 p-3">
            <div className="flex items-center gap-2 mb-3">
                <span className="text-sm font-medium">Rich Content</span>
                <Badge variant="outline" className="text-[10px]">Webhook / Discord / Slack / Teams</Badge>
            </div>
            <Tabs defaultValue="attachments" className="w-full">
                <TabsList className="h-8 w-full justify-start">
                    <TabsTrigger value="attachments" className="text-xs">
                        Attachments {value.attachments.length > 0 && <Badge variant="secondary" className="ml-1 text-[10px]">{value.attachments.length}</Badge>}
                    </TabsTrigger>
                    <TabsTrigger value="actions" className="text-xs">
                        Actions {value.actions.length > 0 && <Badge variant="secondary" className="ml-1 text-[10px]">{value.actions.length}</Badge>}
                    </TabsTrigger>
                    <TabsTrigger value="fields" className="text-xs">
                        Fields {value.fields.length > 0 && <Badge variant="secondary" className="ml-1 text-[10px]">{value.fields.length}</Badge>}
                    </TabsTrigger>
                    <TabsTrigger value="poll" className="text-xs">
                        Poll {value.poll && <Badge variant="secondary" className="ml-1 text-[10px]">1</Badge>}
                    </TabsTrigger>
                    <TabsTrigger value="style" className="text-xs">Style</TabsTrigger>
                    {showJsonPreview && (
                        <TabsTrigger value="json" className="text-xs">Generic preview</TabsTrigger>
                    )}
                </TabsList>

                {/* Attachments Tab */}
                <TabsContent value="attachments" className="mt-2 space-y-2">
                    {value.attachments.map((att, i) => (
                        <div key={i} className="flex gap-2 items-end">
                            <div className="flex-1 space-y-1">
                                <div className="flex gap-2">
                                    <Select value={att.type} onValueChange={v => {
                                        const next = [...value.attachments];
                                        next[i] = { ...next[i], type: v };
                                        update({ attachments: next });
                                    }}>
                                        <SelectTrigger className="w-[100px] h-8 text-xs"><SelectValue /></SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="image">Image</SelectItem>
                                            <SelectItem value="video">Video</SelectItem>
                                            <SelectItem value="file">File</SelectItem>
                                            <SelectItem value="audio">Audio</SelectItem>
                                        </SelectContent>
                                    </Select>
                                    <Input
                                        value={att.url}
                                        onChange={e => {
                                            const next = [...value.attachments];
                                            next[i] = { ...next[i], url: e.target.value };
                                            update({ attachments: next });
                                        }}
                                        placeholder="https://..."
                                        className="h-8 text-xs flex-1"
                                    />
                                    <Input
                                        value={att.alt_text || ''}
                                        onChange={e => {
                                            const next = [...value.attachments];
                                            next[i] = { ...next[i], alt_text: e.target.value };
                                            update({ attachments: next });
                                        }}
                                        placeholder="Alt text"
                                        className="h-8 text-xs w-[120px]"
                                    />
                                </div>
                            </div>
                            <Button type="button" variant="ghost" size="sm" className="h-8" onClick={() => {
                                update({ attachments: value.attachments.filter((_, j) => j !== i) });
                            }}><X className="h-3.5 w-3.5" /></Button>
                        </div>
                    ))}
                    {value.attachments.length < 10 && (
                        <Button type="button" variant="outline" size="sm" className="text-xs" onClick={() => {
                            update({ attachments: [...value.attachments, { type: 'image', url: '' }] });
                        }}><Plus className="h-3.5 w-3.5 mr-1" />Add Attachment</Button>
                    )}
                    <p className="text-[11px] text-muted-foreground">Up to 10 attachments. Images render inline; files as download links.</p>
                </TabsContent>

                {/* Actions Tab */}
                <TabsContent value="actions" className="mt-2 space-y-2">
                    {value.actions.map((act, i) => (
                        <div key={i} className="flex gap-2 items-end">
                            <Select value={act.type} onValueChange={v => {
                                const next = [...value.actions];
                                next[i] = { ...next[i], type: v };
                                update({ actions: next });
                            }}>
                                <SelectTrigger className="w-[90px] h-8 text-xs"><SelectValue /></SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="link">Link</SelectItem>
                                    <SelectItem value="submit">Submit</SelectItem>
                                    <SelectItem value="dismiss">Dismiss</SelectItem>
                                </SelectContent>
                            </Select>
                            <Input
                                value={act.label}
                                onChange={e => {
                                    const next = [...value.actions];
                                    next[i] = { ...next[i], label: e.target.value };
                                    update({ actions: next });
                                }}
                                placeholder="Button label"
                                className="h-8 text-xs w-[120px]"
                            />
                            {act.type === 'link' && (
                                <Input
                                    value={act.url || ''}
                                    onChange={e => {
                                        const next = [...value.actions];
                                        next[i] = { ...next[i], url: e.target.value };
                                        update({ actions: next });
                                    }}
                                    placeholder="https://..."
                                    className="h-8 text-xs flex-1"
                                />
                            )}
                            <Select value={act.style || 'default'} onValueChange={v => {
                                const next = [...value.actions];
                                next[i] = { ...next[i], style: v };
                                update({ actions: next });
                            }}>
                                <SelectTrigger className="w-[90px] h-8 text-xs"><SelectValue /></SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="default">Default</SelectItem>
                                    <SelectItem value="primary">Primary</SelectItem>
                                    <SelectItem value="danger">Danger</SelectItem>
                                </SelectContent>
                            </Select>
                            <Button type="button" variant="ghost" size="sm" className="h-8" onClick={() => {
                                update({ actions: value.actions.filter((_, j) => j !== i) });
                            }}><X className="h-3.5 w-3.5" /></Button>
                        </div>
                    ))}
                    {value.actions.length < 5 && (
                        <Button type="button" variant="outline" size="sm" className="text-xs" onClick={() => {
                            update({ actions: [...value.actions, { type: 'link', label: '', url: '' }] });
                        }}><Plus className="h-3.5 w-3.5 mr-1" />Add Action</Button>
                    )}
                    <p className="text-[11px] text-muted-foreground">Up to 5 buttons. Links render as clickable buttons/URLs per platform.</p>
                </TabsContent>

                {/* Fields Tab */}
                <TabsContent value="fields" className="mt-2 space-y-2">
                    {value.fields.map((fld, i) => (
                        <div key={i} className="flex gap-2 items-center">
                            <Input
                                value={fld.key}
                                onChange={e => {
                                    const next = [...value.fields];
                                    next[i] = { ...next[i], key: e.target.value };
                                    update({ fields: next });
                                }}
                                placeholder="Key"
                                className="h-8 text-xs w-[120px]"
                            />
                            <Input
                                value={fld.value}
                                onChange={e => {
                                    const next = [...value.fields];
                                    next[i] = { ...next[i], value: e.target.value };
                                    update({ fields: next });
                                }}
                                placeholder="Value"
                                className="h-8 text-xs flex-1"
                            />
                            <label className="flex items-center gap-1 text-xs whitespace-nowrap">
                                <Checkbox
                                    checked={fld.inline ?? false}
                                    onCheckedChange={(checked) => {
                                        const next = [...value.fields];
                                        next[i] = { ...next[i], inline: !!checked };
                                        update({ fields: next });
                                    }}
                                />
                                Inline
                            </label>
                            <Button type="button" variant="ghost" size="sm" className="h-8" onClick={() => {
                                update({ fields: value.fields.filter((_, j) => j !== i) });
                            }}><X className="h-3.5 w-3.5" /></Button>
                        </div>
                    ))}
                    {value.fields.length < 25 && (
                        <Button type="button" variant="outline" size="sm" className="text-xs" onClick={() => {
                            update({ fields: [...value.fields, { key: '', value: '', inline: false }] });
                        }}><Plus className="h-3.5 w-3.5 mr-1" />Add Field</Button>
                    )}
                    <p className="text-[11px] text-muted-foreground">Up to 25 key-value fields. Inline fields render side-by-side on supported platforms.</p>
                </TabsContent>

                {/* Poll Tab */}
                <TabsContent value="poll" className="mt-2 space-y-2">
                    <PollEditor
                        poll={value.poll}
                        onChange={(poll) => update({ poll })}
                    />
                </TabsContent>

                {/* Style Tab */}
                <TabsContent value="style" className="mt-2 space-y-3">
                    <div className="space-y-1.5">
                        <Label className="text-xs">Severity</Label>
                        <Select
                            value={value.style?.severity || 'none'}
                            onValueChange={v => {
                                if (v === 'none') {
                                    update({ style: value.style?.color ? { color: value.style.color } : null });
                                } else {
                                    update({ style: { ...value.style, severity: v as any } });
                                }
                            }}
                        >
                            <SelectTrigger className="h-8 text-xs w-[160px]"><SelectValue /></SelectTrigger>
                            <SelectContent>
                                <SelectItem value="none">None</SelectItem>
                                <SelectItem value="info">Info (blue)</SelectItem>
                                <SelectItem value="success">Success (green)</SelectItem>
                                <SelectItem value="warning">Warning (orange)</SelectItem>
                                <SelectItem value="danger">Danger (red)</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-1.5">
                        <Label className="text-xs">Color Override (hex)</Label>
                        <Input
                            value={value.style?.color || ''}
                            onChange={e => {
                                const color = e.target.value;
                                if (!color && !value.style?.severity) {
                                    update({ style: null });
                                } else {
                                    update({ style: { ...value.style, color: color || undefined } });
                                }
                            }}
                            placeholder="#3498DB"
                            className="h-8 text-xs w-[160px] font-mono"
                        />
                    </div>
                </TabsContent>

                {/* JSON Preview */}
                {showJsonPreview && (
                    <TabsContent value="json" className="mt-2 space-y-2">
                        <div className="rounded-md border border-border bg-background/60 p-3">
                            <div className="text-[11px] text-muted-foreground mb-2">
                                This is the exact shape sent to the API as top-level rich fields (not inside template data).
                            </div>
                            <pre className="max-h-[260px] overflow-auto rounded bg-black/90 p-3 text-[11px] text-green-200">
{JSON.stringify(payloadPreview, null, 2)}
                            </pre>
                        </div>
                    </TabsContent>
                )}
            </Tabs>
        </div>
    );
};

// ── Poll sub-editor ──

interface PollEditorProps {
    poll: ContentPoll | null;
    onChange: (poll: ContentPoll | null) => void;
}

const PollEditor: React.FC<PollEditorProps> = ({ poll, onChange }) => {
    if (!poll) {
        return (
            <div className="text-center py-3">
                <p className="text-xs text-muted-foreground mb-2">No poll configured.</p>
                <Button type="button" variant="outline" size="sm" className="text-xs" onClick={() => {
                    onChange({ question: '', choices: [{ label: '' }, { label: '' }] });
                }}><Plus className="h-3.5 w-3.5 mr-1" />Add Poll</Button>
            </div>
        );
    }

    return (
        <div className="space-y-2">
            <div className="flex justify-between items-center">
                <Label className="text-xs">Question</Label>
                <Button type="button" variant="ghost" size="sm" className="text-xs text-destructive h-6" onClick={() => onChange(null)}>
                    Remove Poll
                </Button>
            </div>
            <Input
                value={poll.question}
                onChange={e => onChange({ ...poll, question: e.target.value })}
                placeholder="What do you think?"
                className="h-8 text-xs"
            />
            <Label className="text-xs">Choices (2-10)</Label>
            {poll.choices.map((ch, i) => (
                <div key={i} className="flex gap-2 items-center">
                    <Input
                        value={ch.emoji || ''}
                        onChange={e => {
                            const next = [...poll.choices];
                            next[i] = { ...next[i], emoji: e.target.value };
                            onChange({ ...poll, choices: next });
                        }}
                        placeholder="🎉"
                        className="h-8 text-xs w-[50px]"
                    />
                    <Input
                        value={ch.label}
                        onChange={e => {
                            const next = [...poll.choices];
                            next[i] = { ...next[i], label: e.target.value };
                            onChange({ ...poll, choices: next });
                        }}
                        placeholder={`Choice ${i + 1}`}
                        className="h-8 text-xs flex-1"
                    />
                    {poll.choices.length > 2 && (
                        <Button type="button" variant="ghost" size="sm" className="h-8" onClick={() => {
                            onChange({ ...poll, choices: poll.choices.filter((_, j) => j !== i) });
                        }}><X className="h-3.5 w-3.5" /></Button>
                    )}
                </div>
            ))}
            {poll.choices.length < 10 && (
                <Button type="button" variant="outline" size="sm" className="text-xs" onClick={() => {
                    onChange({ ...poll, choices: [...poll.choices, { label: '' }] });
                }}><Plus className="h-3.5 w-3.5 mr-1" />Add Choice</Button>
            )}
            <label className="flex items-center gap-2 text-xs">
                <Checkbox
                    checked={poll.multi_select ?? false}
                    onCheckedChange={(checked) => onChange({ ...poll, multi_select: !!checked })}
                />
                Allow multiple selections
            </label>
        </div>
    );
};

export default RichContentEditor;
