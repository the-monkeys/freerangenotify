import React, { useState, useCallback, useMemo, useEffect } from 'react';
import {
    DndContext,
    closestCenter,
    KeyboardSensor,
    PointerSensor,
    useSensor,
    useSensors,
    type DragEndEvent,
} from '@dnd-kit/core';
import {
    arrayMove,
    SortableContext,
    sortableKeyboardCoordinates,
    useSortable,
    verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { GripVertical, Type, Heading2, Image, MousePointer, Minus, Trash2 } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import { cn } from '@/lib/utils';

// ── Block types ──

export type BlockType = 'text' | 'heading' | 'image' | 'button' | 'divider';

export interface EmailBlock {
    id: string;
    type: BlockType;
    data: Record<string, string>;
}

const DEFAULT_BLOCKS: Record<BlockType, EmailBlock['data']> = {
    text: { content: '<p>Hello {{.name}}, welcome!</p>' },
    heading: { level: 'h2', content: 'Heading' },
    image: { src: '', alt: 'Image', width: '600' },
    button: { text: 'Click here', url: 'https://example.com' },
    divider: {},
};

function createBlock(type: BlockType, id?: string): EmailBlock {
    return {
        id: id ?? `block-${Date.now()}-${Math.random().toString(36).slice(2)}`,
        type,
        data: { ...DEFAULT_BLOCKS[type] },
    };
}

// ── Serialize blocks to HTML ──

function blocksToHtml(blocks: EmailBlock[]): string {
    const parts: string[] = [];
    for (const b of blocks) {
        switch (b.type) {
            case 'text':
                parts.push(b.data.content || '<p></p>');
                break;
            case 'heading': {
                const tag = (b.data.level || 'h2') as 'h1' | 'h2' | 'h3';
                parts.push(`<${tag}>${escapeHtml(b.data.content || '')}</${tag}>`);
                break;
            }
            case 'image': {
                if (!b.data.src) break;
                const w = b.data.width ? ` width="${escapeHtml(b.data.width)}"` : '';
                parts.push(`<img src="${escapeHtml(b.data.src)}" alt="${escapeHtml(b.data.alt || '')}"${w} style="max-width:100%;height:auto;" />`);
                break;
            }
            case 'button': {
                const text = b.data.text || 'Click';
                const url = b.data.url || '#';
                parts.push(`<p style="margin:16px 0;"><a href="${escapeHtml(url)}" style="display:inline-block;padding:12px 24px;background:#2563eb;color:white;text-decoration:none;border-radius:6px;">${escapeHtml(text)}</a></p>`);
                break;
            }
            case 'divider':
                parts.push('<hr style="border:none;border-top:1px solid #e5e7eb;margin:24px 0;" />');
                break;
        }
    }
    return parts.join('\n');
}

function escapeHtml(s: string): string {
    return s
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

// ── Parse HTML into blocks (best-effort) ──

function htmlToBlocks(html: string): EmailBlock[] {
    if (!html?.trim()) return [createBlock('text')];
    const trimmed = html.trim();
    if (trimmed.length < 2) return [createBlock('text')];
    // Wrap existing HTML in a single text block; a full parser would split by block type
    return [{
        id: `block-${Date.now()}-0`,
        type: 'text',
        data: { content: trimmed },
    }];
}

// ── Sortable block item ──

interface SortableBlockItemProps {
    block: EmailBlock;
    variables: string[];
    onUpdate: (id: string, data: Record<string, string>) => void;
    onRemove: (id: string) => void;
}

function SortableBlockItem({ block, variables, onUpdate, onRemove }: SortableBlockItemProps) {
    const {
        attributes,
        listeners,
        setNodeRef,
        transform,
        transition,
        isDragging,
    } = useSortable({ id: block.id });

    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
    };

    return (
        <div
            ref={setNodeRef}
            style={style}
            className={cn(
                'group flex gap-2 items-start p-3 rounded-lg border bg-card hover:border-primary/40 transition-colors',
                isDragging && 'opacity-80 shadow-lg z-10'
            )}
        >
            <button
                type="button"
                className="mt-1 p-1 rounded hover:bg-muted cursor-grab active:cursor-grabbing text-muted-foreground"
                {...attributes}
                {...listeners}
                aria-label="Drag to reorder"
            >
                <GripVertical className="size-4" />
            </button>
            <div className="flex-1 min-w-0">
                <BlockEditor block={block} variables={variables} onUpdate={(d) => onUpdate(block.id, d)} />
            </div>
            <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive shrink-0"
                onClick={() => onRemove(block.id)}
                aria-label="Remove block"
            >
                <Trash2 className="size-4" />
            </Button>
        </div>
    );
}

// ── Block editor (content per type) ──

interface BlockEditorProps {
    block: EmailBlock;
    variables: string[];
    onUpdate: (data: Record<string, string>) => void;
}

function BlockEditor({ block, variables, onUpdate }: BlockEditorProps) {
    const update = (key: string, value: string) => {
        onUpdate({ ...block.data, [key]: value });
    };

    const insertVariable = (v: string) => {
        const placeholder = `{{.${v}}}`;
        if (block.type === 'text' || block.type === 'heading') {
            update('content', (block.data.content || '') + placeholder);
        } else if (block.type === 'button') {
            update('text', (block.data.text || '') + placeholder);
        }
    };

    switch (block.type) {
        case 'text':
            return (
                <div className="space-y-1">
                    <Textarea
                        className="min-h-[80px] font-sans text-sm font-mono"
                        value={block.data.content || ''}
                        onChange={(e) => update('content', e.target.value)}
                        placeholder="<p>Hello {{.name}}, welcome!</p>"
                    />
                    {variables.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                            {variables.map((v) => (
                                <Button key={v} type="button" variant="outline" size="sm" className="text-xs h-6 px-2"
                                    onClick={() => insertVariable(v)}>
                                    {`{{.${v}}}`}
                                </Button>
                            ))}
                        </div>
                    )}
                </div>
            );
        case 'heading':
            return (
                <div className="space-y-2">
                    <select
                        className="h-8 rounded border bg-background px-2 text-sm"
                        value={block.data.level || 'h2'}
                        onChange={(e) => update('level', e.target.value)}
                    >
                        <option value="h1">Heading 1</option>
                        <option value="h2">Heading 2</option>
                        <option value="h3">Heading 3</option>
                    </select>
                    <Input
                        value={block.data.content || ''}
                        onChange={(e) => update('content', e.target.value)}
                        placeholder="Heading text"
                    />
                    {variables.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                            {variables.map((v) => (
                                <Button key={v} type="button" variant="outline" size="sm" className="text-xs h-6 px-2"
                                    onClick={() => update('content', (block.data.content || '') + `{{.${v}}}`)}>
                                    {`{{.${v}}}`}
                                </Button>
                            ))}
                        </div>
                    )}
                </div>
            );
        case 'image':
            return (
                <div className="space-y-2">
                    <Input
                        placeholder="Image URL (https://...)"
                        value={block.data.src || ''}
                        onChange={(e) => update('src', e.target.value)}
                    />
                    <Input
                        placeholder="Alt text"
                        value={block.data.alt || ''}
                        onChange={(e) => update('alt', e.target.value)}
                    />
                    <Input
                        placeholder="Width (px)"
                        value={block.data.width || ''}
                        onChange={(e) => update('width', e.target.value)}
                    />
                </div>
            );
        case 'button':
            return (
                <div className="space-y-2">
                    <Input
                        placeholder="Button text"
                        value={block.data.text || ''}
                        onChange={(e) => update('text', e.target.value)}
                    />
                    <Input
                        placeholder="Button URL"
                        value={block.data.url || ''}
                        onChange={(e) => update('url', e.target.value)}
                    />
                    {variables.length > 0 && (
                        <div className="flex flex-wrap gap-1">
                            {variables.map((v) => (
                                <Button key={v} type="button" variant="outline" size="sm" className="text-xs h-6 px-2"
                                    onClick={() => update('text', (block.data.text || '') + `{{.${v}}}`)}>
                                    {`{{.${v}}}`}
                                </Button>
                            ))}
                        </div>
                    )}
                </div>
            );
        case 'divider':
            return <p className="text-xs text-muted-foreground py-1">Horizontal divider</p>;
        default:
            return null;
    }
}

// ── Block palette ──

const BLOCK_TYPES: { type: BlockType; label: string; icon: React.ReactNode }[] = [
    { type: 'text', label: 'Text', icon: <Type className="size-4" /> },
    { type: 'heading', label: 'Heading', icon: <Heading2 className="size-4" /> },
    { type: 'image', label: 'Image', icon: <Image className="size-4" /> },
    { type: 'button', label: 'Button', icon: <MousePointer className="size-4" /> },
    { type: 'divider', label: 'Divider', icon: <Minus className="size-4" /> },
];

// ── Main component ──

export interface EmailBlockBuilderProps {
    content: string;
    onChange: (html: string) => void;
    variables?: string[];
    placeholder?: string;
}

export function EmailBlockBuilder({ content, onChange, variables = [] }: EmailBlockBuilderProps) {
    const [blocks, setBlocks] = useState<EmailBlock[]>(() => {
        if (content?.trim()) return htmlToBlocks(content);
        return [createBlock('text')];
    });

    // Sync from external content when parent provides different HTML (e.g. switching from HTML mode)
    useEffect(() => {
        if (!content?.trim()) return;
        const currentHtml = blocksToHtml(blocks);
        if (content.trim() !== currentHtml.trim()) {
            setBlocks(htmlToBlocks(content));
        }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only when parent content changes
    }, [content]);

    const sensors = useSensors(
        useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
        useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
    );

    const addBlock = useCallback((type: BlockType) => {
        setBlocks((prev) => {
            const next = [...prev, createBlock(type)];
            onChange(blocksToHtml(next));
            return next;
        });
    }, [onChange]);

    const updateBlock = useCallback((id: string, data: Record<string, string>) => {
        setBlocks((prev) => {
            const next = prev.map((b) => (b.id === id ? { ...b, data } : b));
            onChange(blocksToHtml(next));
            return next;
        });
    }, [onChange]);

    const removeBlock = useCallback((id: string) => {
        setBlocks((prev) => {
            const next = prev.filter((b) => b.id !== id);
            onChange(blocksToHtml(next.length ? next : [createBlock('text')]));
            return next.length ? next : [createBlock('text')];
        });
    }, [onChange]);

    const handleDragEnd = useCallback((event: DragEndEvent) => {
        const { active, over } = event;
        if (over && active.id !== over.id) {
            setBlocks((prev) => {
                const oldIndex = prev.findIndex((b) => b.id === active.id);
                const newIndex = prev.findIndex((b) => b.id === over.id);
                if (oldIndex === -1 || newIndex === -1) return prev;
                const next = arrayMove(prev, oldIndex, newIndex);
                onChange(blocksToHtml(next));
                return next;
            });
        }
    }, [onChange]);

    const blockIds = useMemo(() => blocks.map((b) => b.id), [blocks]);

    return (
        <div className="space-y-4">
            {/* Block palette */}
            <div className="flex flex-wrap gap-2 p-2 rounded-lg border bg-muted/30">
                <span className="text-xs text-muted-foreground self-center mr-2">Add block:</span>
                {BLOCK_TYPES.map(({ type, label, icon }) => (
                    <Button
                        key={type}
                        type="button"
                        variant="outline"
                        size="sm"
                        className="gap-2"
                        onClick={() => addBlock(type)}
                    >
                        {icon}
                        {label}
                    </Button>
                ))}
            </div>

            {/* Sortable block list */}
            <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
                <SortableContext items={blockIds} strategy={verticalListSortingStrategy}>
                    <div className="space-y-2">
                        {blocks.map((block) => (
                            <SortableBlockItem
                                key={block.id}
                                block={block}
                                variables={variables}
                                onUpdate={updateBlock}
                                onRemove={removeBlock}
                            />
                        ))}
                    </div>
                </SortableContext>
            </DndContext>
        </div>
    );
}
