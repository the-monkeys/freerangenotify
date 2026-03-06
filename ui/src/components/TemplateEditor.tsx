import React, { useState, useRef, useCallback, useEffect } from 'react';
import { Button } from './ui/button';
import { Textarea } from './ui/textarea';

interface TemplateEditorProps {
    content: string;
    onChange: (html: string) => void;
    channel: string;
    placeholder?: string;
}

/**
 * Rich template editor for email channels with visual/HTML toggle,
 * formatting toolbar, and live preview. Falls back to plain textarea
 * for non-email channels (SMS, push, webhook, SSE).
 */
const TemplateEditor: React.FC<TemplateEditorProps> = ({ content, onChange, channel, placeholder }) => {
    const [mode, setMode] = useState<'visual' | 'html'>('visual');
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
            <Textarea
                className="min-h-[200px] font-mono text-sm"
                value={content}
                onChange={(e) => onChange(e.target.value)}
                placeholder={placeholder || 'Write your template content...'}
            />
        );
    }

    return (
        <div className="space-y-2">
            <div className="border rounded overflow-hidden">
                {/* Toolbar */}
                <div className="flex items-center gap-1 p-2 border-b bg-muted/30 flex-wrap">
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
                                }}>🖼 Image</Button>
                        </>
                    )}
                </div>

                {/* Editor area */}
                {mode === 'visual' ? (
                    <div
                        ref={editorRef}
                        contentEditable
                        suppressContentEditableWarning
                        className="p-4 min-h-[300px] prose max-w-none focus:outline-none bg-white"
                        onInput={handleEditorInput}
                    />
                ) : (
                    <textarea
                        className="w-full min-h-[300px] font-mono text-sm p-4 border-0 focus:outline-none resize-y"
                        value={content}
                        onChange={handleHtmlChange}
                        placeholder={placeholder || 'Enter raw HTML...'}
                    />
                )}
            </div>

            {/* Live preview for email */}
            {content && (
                <div>
                    <p className="text-xs text-gray-500 font-semibold mb-1 uppercase">Live Preview</p>
                    <iframe
                        srcDoc={content}
                        className="w-full h-[300px] border rounded bg-white"
                        sandbox=""
                        title="Email Template Preview"
                    />
                </div>
            )}
        </div>
    );
};

export default TemplateEditor;
