import React, { useState, useRef, useCallback, useEffect } from 'react';
import { Button } from './ui/button';
import { Textarea } from './ui/textarea';
import { EmailBlockBuilder } from './templates/EmailBlockBuilder';
import { Badge } from './ui/badge';

interface TemplateEditorProps {
    content: string;
    onChange: (html: string) => void;
    channel: string;
    placeholder?: string;
    /** Variable names for the block builder's variable inserter. Only used when channel is email. */
    variables?: string[];
}

/**
 * Rich template editor for email channels with visual/HTML toggle,
 * formatting toolbar, and live preview. Falls back to plain textarea
 * for non-email channels (SMS, push, webhook, SSE).
 */
const TemplateEditor: React.FC<TemplateEditorProps> = ({ content, onChange, channel, placeholder, variables = [] }) => {
    const [mode, setMode] = useState<'builder' | 'visual' | 'html'>('builder');
    const editorRef = useRef<HTMLDivElement>(null);
    // Track whether we're actively typing in the contentEditable div
    // to prevent dangerouslySetInnerHTML from resetting cursor position.
    const isComposingRef = useRef(false);

    const execCommand = useCallback((command: string, value?: string) => {
        document.execCommand(command, false, value);
        if (editorRef.current) {
            onChange(editorRef.current.innerHTML);
        }
    }, [onChange]);

    const handleEditorInput = useCallback(() => {
        if (editorRef.current) {
            isComposingRef.current = true;
            onChange(editorRef.current.innerHTML);
        }
    }, [onChange]);

    const handleHtmlChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
        onChange(e.target.value);
    }, [onChange]);

    const switchToVisual = useCallback(() => {
        setMode('visual');
    }, []);

    const switchToHtml = useCallback(() => {
        if (editorRef.current) {
            // Sync final visual content before switching
            onChange(editorRef.current.innerHTML);
        }
        setMode('html');
    }, [onChange]);

    // Sync content into the contentEditable div only when:
    // - Switching to visual mode (mode changes)
    // - External content changes (e.g. from editing template, restoring version)
    // NOT on every keystroke (that causes the cursor-reset / backward-text bug).
    useEffect(() => {
        if (mode === 'visual' && editorRef.current) {
            if (isComposingRef.current) {
                // User is actively typing — skip overwriting innerHTML
                isComposingRef.current = false;
                return;
            }
            if (editorRef.current.innerHTML !== content) {
                editorRef.current.innerHTML = content;
            }
        }
    }, [mode, content]);

    // For non-email channels, plain textarea is sufficient.
    // Placed AFTER all hooks to comply with React's Rules of Hooks.
    if (channel !== 'email') {
        return (
            <div className="space-y-2">
                <div className="rounded-md border border-border/70 bg-muted/35 px-3 py-2 text-xs text-muted-foreground">
                    Plain text editor for this channel.
                </div>
                <Textarea
                    className="min-h-55 font-mono text-sm"
                    value={content}
                    onChange={(e) => onChange(e.target.value)}
                    placeholder={placeholder || 'Write your template content...'}
                />
            </div>
        );
    }

    // Email channel: Builder (drag-and-drop blocks), Visual (contentEditable), or HTML
    return (
        <div className="space-y-3">
            <div className="rounded-lg border border-border/80 overflow-hidden bg-background">
                {/* Toolbar */}
                <div className="flex items-center gap-1.5 p-2 border-b bg-muted/25 flex-wrap">
                    <Button
                        type="button"
                        variant={mode === 'builder' ? 'default' : 'ghost'}
                        size="sm"
                        onClick={() => setMode('builder')}
                    >
                        Builder
                    </Button>
                    <Button
                        type="button"
                        variant={mode === 'visual' ? 'default' : 'ghost'}
                        size="sm"
                        onClick={switchToVisual}
                    >
                        Visual
                    </Button>
                    <Button
                        type="button"
                        variant={mode === 'html' ? 'default' : 'ghost'}
                        size="sm"
                        onClick={switchToHtml}
                    >
                        HTML
                    </Button>
                    <Badge variant="outline" className="ml-auto text-[11px]">
                        Email editor
                    </Badge>

                    {mode === 'visual' && (
                        <>
                            <div className="w-px h-6 bg-border mx-1" />
                            <Button type="button" variant="ghost" size="sm" className="font-bold px-2"
                                onClick={() => execCommand('bold')}>B</Button>
                            <Button type="button" variant="ghost" size="sm" className="italic px-2"
                                onClick={() => execCommand('italic')}>I</Button>
                            <Button type="button" variant="ghost" size="sm" className="underline px-2"
                                onClick={() => execCommand('underline')}>U</Button>
                            <div className="w-px h-6 bg-border mx-1" />
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => execCommand('formatBlock', 'h2')}>H2</Button>
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => execCommand('formatBlock', 'h3')}>H3</Button>
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => execCommand('formatBlock', 'p')}>P</Button>
                            <div className="w-px h-6 bg-border mx-1" />
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => execCommand('insertUnorderedList')}>• List</Button>
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => execCommand('insertOrderedList')}>1. List</Button>
                            <div className="w-px h-6 bg-border mx-1" />
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => {
                                    const url = prompt('Enter link URL:');
                                    if (url) execCommand('createLink', url);
                                }}>Link</Button>
                            <Button type="button" variant="ghost" size="sm"
                                onClick={() => {
                                    const url = prompt('Image URL (https://...):');
                                    if (!url) return;
                                    const alt = prompt('Alt text (optional):') || '';
                                    const width = prompt('Width in px (optional, e.g. 600):') || '';
                                    const widthAttr = width ? ` width="${width}"` : '';
                                    const img = `<img src="${url}" alt="${alt}"${widthAttr} style="max-width:100%;height:auto;" />`;
                                    execCommand('insertHTML', img);
                                }}>Image</Button>
                        </>
                    )}
                </div>

                {/* Editor area */}
                {mode === 'builder' ? (
                    <div className="p-4 min-h-75 bg-white">
                        <EmailBlockBuilder
                            content={content}
                            onChange={onChange}
                            variables={variables}
                            placeholder={placeholder}
                        />
                    </div>
                ) : mode === 'visual' ? (
                    <div
                        ref={editorRef}
                        contentEditable
                        suppressContentEditableWarning
                        className="p-4 min-h-75 prose max-w-none focus:outline-none bg-white"
                        onInput={handleEditorInput}
                    />
                ) : (
                    <Textarea
                        className="w-full min-h-75 rounded-none border-0 font-mono text-sm p-4 resize-y focus-visible:ring-0"
                        value={content}
                        onChange={handleHtmlChange}
                        placeholder={placeholder || 'Enter raw HTML...'}
                    />
                )}
            </div>

            {/* Live preview for email */}
            {content && (
                <div className="rounded-lg border border-border/80 bg-background p-3">
                    <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Live preview</p>
                    <iframe
                        srcDoc={content}
                        className="w-full h-75 rounded-md border bg-white"
                        sandbox=""
                        title="Email Template Preview"
                    />
                </div>
            )}
        </div>
    );
};

export default TemplateEditor;
