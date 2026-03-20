import React, { useEffect, useCallback, useState } from 'react';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Separator } from '../ui/separator';
import { Camera, Mic, MoreVertical, Paperclip, Phone, Smile, Video } from 'lucide-react';
import type { Template } from '../../types';

interface EditablePreviewPanelProps {
    slidePreview: { templateId: string; templateName: string; channel: string } | null;
    templates: Template[];
    activePreviews: Record<string, { data: string; rendered: string; loading: boolean }>;
    savingDefaults: Record<string, boolean>;
    showDefaultActions?: boolean;
    onClose: () => void;
    onRenderPreview: (templateId: string) => void;
    onSaveDefaults: (template: Template) => void;
    onResetDefaults: (template: Template) => void;
    onVariableEdit: (templateId: string, variable: string, value: string) => void;
}

/** CSS injected into the preview iframe to style contenteditable variable spans. */
const EDITABLE_STYLE = `<style>
.frn-editable {
    outline: none;
    transition: outline 0.15s, background-color 0.15s;
    cursor: text;
    border-radius: 2px;
    min-width: 1ch;
    display: inline;
}
.frn-editable:hover {
    outline: 2px dashed #3b82f6;
    background-color: rgba(59, 130, 246, 0.05);
}
.frn-editable:focus {
    outline: 2px solid #3b82f6;
    background-color: rgba(59, 130, 246, 0.08);
}
</style>`;

/** Script injected into the preview iframe to relay contenteditable changes back to the parent. */
const EDITABLE_SCRIPT = `<script>
document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('[data-frn-var]').forEach(function(el) {
        el.addEventListener('input', function() {
            window.parent.postMessage({
                type: 'frn-var-edit',
                variable: el.getAttribute('data-frn-var'),
                value: el.textContent || ''
            }, '*');
        });
    });
});
<\/script>`;

/**
 * Sorts template variables by the order they first appear in the template body.
 * Variables not found in the body are appended at the end.
 */
function sortVariablesByAppearance(variables: string[], body: string): string[] {
    return [...variables].sort((a, b) => {
        const posA = body.indexOf(`{{${a}}}`);
        const posB = body.indexOf(`{{${b}}}`);
        const ia = posA === -1 ? Infinity : posA;
        const ib = posB === -1 ? Infinity : posB;
        return ia - ib;
    });
}

/**
 * Infers the display type for a variable based on its name.
 * Used to render an appropriate input widget in the sidebar.
 */
function inferVariableType(name: string): 'image' | 'url' | 'text' {
    const n = name.toLowerCase();
    if (/image|img|logo|photo|avatar|thumbnail|banner|picture/.test(n)) return 'image';
    if (/url|link|href|src|uri|website/.test(n)) return 'url';
    return 'text';
}

/**
 * Injects editable CSS and communication script into the rendered HTML
 * so that contenteditable variable spans are styled and report changes.
 */
function injectEditableSupport(html: string): string {
    let result = html;

    // Inject style into <head> or prepend
    if (result.includes('</head>')) {
        result = result.replace('</head>', EDITABLE_STYLE + '</head>');
    } else {
        result = EDITABLE_STYLE + result;
    }

    // Inject script before </body> or append
    if (result.includes('</body>')) {
        result = result.replace('</body>', EDITABLE_SCRIPT + '</body>');
    } else {
        result = result + EDITABLE_SCRIPT;
    }

    return result;
}

function extractWhatsAppBody(rendered: string): string {
    if (!rendered) return '';

    const bodyMatch = rendered.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
    const fromBody = bodyMatch?.[1] ?? rendered;

    // Keep preview safe and deterministic in the panel.
    return fromBody
        .replace(/<script[\s\S]*?<\/script>/gi, '')
        .replace(/<style[\s\S]*?<\/style>/gi, '')
        .trim();
}

const WhatsAppMobilePreview: React.FC<{ rendered: string; mediaUrl?: string }> = ({ rendered, mediaUrl }) => {
    const timeString = new Date().toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        hour12: true,
    });

    const applicationName = localStorage.getItem('last_app_name') || 'FreeRange Notify';

    const bubbleHtml = extractWhatsAppBody(rendered);
    const hasHtml = /<[^>]+>/.test(bubbleHtml);
    const showMedia = !!mediaUrl && /^(https?:)?\/\//i.test(mediaUrl);

    return (
        <div className="h-full p-4 sm:p-6 overflow-y-auto bg-muted/20">
            <div className="mx-auto w-full max-w-sm h-full rounded-lg flex flex-col border border-border overflow-hidden bg-[#efe7de] dark:bg-[#0b141a]">
                <div
                    className="absolute inset-0 opacity-20 dark:opacity-10 pointer-events-none"
                    style={{
                        backgroundImage:
                            'radial-gradient(circle at 20% 30%, rgba(0,0,0,0.08) 0, transparent 32%), radial-gradient(circle at 70% 65%, rgba(0,0,0,0.07) 0, transparent 28%)',
                    }}
                />

                <div className="relative z-10 bg-[#f0f2f5] dark:bg-[#202c33] px-3 py-2.5 border-b border-black/10 dark:border-white/10">
                    <div className="flex items-center gap-2.5">
                        <div className="size-9 rounded-full bg-emerald-600 text-white flex items-center justify-center text-xl">
                            {applicationName.substring(0, 1).toUpperCase()}
                        </div>
                        <div className="min-w-0 flex-1">
                            <p className="text-sm font-semibold text-[#111b21] dark:text-[#e9edef] truncate">{applicationName}</p>
                            <p className="text-[11px] text-[#667781] dark:text-[#8696a0]">Business Account</p>
                        </div>
                        <div className='flex flex-row items-center gap-4'>
                            <Video className="size-6 text-[#667781] dark:text-[#8696a0]" />
                            <Phone className="size-5 text-[#667781] dark:text-[#8696a0]" />
                            <MoreVertical className="size-5 text-[#667781] dark:text-[#8696a0]" />
                        </div>

                    </div>
                </div>

                <div className="px-3 py-3 flex flex-col flex-1">
                    <div className="self-center mb-3 rounded-md bg-white/80 dark:bg-[#182229] px-2 py-0.5 text-[10px] uppercase font-semibold tracking-wide text-[#667781] dark:text-[#8696a0]">
                        Today
                    </div>

                    {showMedia && (
                        <div className="relative mb-2 max-w-[86%] rounded-2xl rounded-tl-sm bg-white dark:bg-[#202c33] shadow p-1.5">
                            <img src={mediaUrl} alt="WhatsApp media" className="rounded-xl max-h-56 w-full object-cover" />
                            <div className="mt-1 text-right text-[10px] text-[#667781] dark:text-[#8696a0]">{timeString}</div>
                        </div>
                    )}

                    <div className="relative max-w-[86%] rounded-2xl rounded-tl-sm bg-white dark:bg-[#202c33] shadow px-3 py-2 text-[14px] leading-5 text-[#111b21] dark:text-[#e9edef]">
                        {hasHtml ? (
                            <div
                                className="[&_a]:text-emerald-600 [&_a]:underline [&_strong]:font-semibold [&_p]:my-0"
                                dangerouslySetInnerHTML={{ __html: bubbleHtml }}
                            />
                        ) : (
                            <p className="whitespace-pre-wrap">{bubbleHtml || 'No WhatsApp content rendered.'}</p>
                        )}
                        <div className="mt-1 text-right text-[10px] text-[#667781] dark:text-[#8696a0]">{timeString}</div>
                    </div>
                </div>

                <div className="bg-[#f0f2f5] dark:bg-[#202c33] p-2 border-t border-black/10 dark:border-white/10">
                    <div className="flex items-center gap-1.5">
                        <div className="flex-1 rounded-full bg-white dark:bg-[#2a3942] px-3 py-2 flex items-center gap-2 text-[#667781] dark:text-[#8696a0]">
                            <Smile className="h-4.5 w-4.5" />
                            <span className="text-xs flex-1">Message</span>
                            <Paperclip className="h-4 w-4 -rotate-45" />
                            <Camera className="h-4 w-4" />
                        </div>
                        <div className="rounded-full bg-[#00a884] p-2 text-white">
                            <Mic className="h-4 w-4" />
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
};

const EditablePreviewPanel: React.FC<EditablePreviewPanelProps> = ({
    slidePreview,
    templates,
    activePreviews,
    savingDefaults,
    showDefaultActions = true,
    onClose,
    onRenderPreview,
    onSaveDefaults,
    onResetDefaults,
    onVariableEdit,
}) => {
    const currentTemplate = slidePreview
        ? templates.find((t) => t.id === slidePreview.templateId)
        : null;
    const currentPreview = slidePreview ? activePreviews[slidePreview.templateId] : undefined;

    // Parse the current variable values from the stored JSON data string.
    const parsedVariables: Record<string, string> = (() => {
        try {
            return JSON.parse(currentPreview?.data || '{}');
        } catch {
            return {};
        }
    })();
    const whatsappMediaUrl =
        parsedVariables.media_url ||
        parsedVariables.image_url ||
        parsedVariables.image ||
        parsedVariables.img_url ||
        '';

    const rawVariables = currentTemplate?.variables ?? [];
    const hasVariables = rawVariables.length > 0;
    const variables = currentTemplate?.body
        ? sortVariablesByAppearance(rawVariables, currentTemplate.body)
        : rawVariables;

    const [showVariables, setShowVariables] = useState(false);

    // Listen for inline-edit messages from the preview iframe.
    const handleMessage = useCallback(
        (e: MessageEvent) => {
            if (
                slidePreview &&
                e.data?.type === 'frn-var-edit' &&
                typeof e.data.variable === 'string'
            ) {
                onVariableEdit(slidePreview.templateId, e.data.variable, e.data.value ?? '');
            }
        },
        [slidePreview?.templateId, onVariableEdit],
    );

    useEffect(() => {
        if (!slidePreview) return;
        window.addEventListener('message', handleMessage);
        return () => window.removeEventListener('message', handleMessage);
    }, [slidePreview, handleMessage]);

    return (
        <SlidePanel
            open={!!slidePreview}
            onClose={onClose}
            title={slidePreview ? `Rendered: ${slidePreview.templateName}` : 'Preview'}
        >
            {slidePreview ? (
                <div className="flex flex-col h-full -m-5">
                    {/* Compact toolbar */}
                    <div className="flex items-center gap-2 px-4 py-2.5 border-b border-border bg-muted/30 shrink-0">
                        {!hasVariables && (
                            <p className="text-xs text-muted-foreground flex-1">
                                Click any <span className="text-blue-500 font-medium">highlighted</span> text in the preview to edit it directly.
                            </p>
                        )}
                        {hasVariables && (
                            <button
                                onClick={() => setShowVariables((v) => !v)}
                                className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors flex-1 text-left"
                            >
                                <span
                                    className={`inline-block transition-transform duration-200 ${showVariables ? 'rotate-0' : '-rotate-90'}`}
                                >
                                    ▾
                                </span>
                                {showVariables ? 'Hide Variables' : 'Show Variables'}
                            </button>
                        )}
                        <Button
                            size="sm"
                            className="text-xs h-7"
                            onClick={() => onRenderPreview(slidePreview.templateId)}
                            disabled={!!currentPreview?.loading}
                        >
                            {currentPreview?.loading ? 'Rendering...' : 'Re-render'}
                        </Button>
                        {showDefaultActions && currentTemplate && (
                            <Button
                                size="sm"
                                variant="outline"
                                className="text-xs h-7"
                                onClick={() => onSaveDefaults(currentTemplate)}
                                disabled={!!savingDefaults[currentTemplate.id]}
                            >
                                {savingDefaults[currentTemplate.id] ? 'Saving...' : 'Save Defaults'}
                            </Button>
                        )}
                        {showDefaultActions && currentTemplate && (
                            <Button
                                size="sm"
                                variant="ghost"
                                className="text-xs h-7 text-muted-foreground"
                                onClick={() => onResetDefaults(currentTemplate)}
                                disabled={!!savingDefaults[currentTemplate.id]}
                            >
                                Reset
                            </Button>
                        )}
                    </div>

                    {/* Body: optional variables sidebar + preview */}
                    <div className="flex flex-1 min-h-0 overflow-hidden">
                        {/* Variables sidebar */}
                        {hasVariables && showVariables && (
                            <>
                                <div className="w-72 shrink-0 flex flex-col overflow-y-auto border-r border-border bg-muted/10">
                                    <div className="px-4 py-3 shrink-0">
                                        <p className="text-xs font-semibold text-foreground uppercase tracking-wide">Variables</p>
                                        <p className="text-xs text-muted-foreground mt-0.5">
                                            Edit values then click <span className="font-medium">Re-render</span>.
                                        </p>
                                    </div>
                                    <Separator />
                                    <div className="flex flex-col gap-4 px-4 py-3">
                                        {variables.map((varName) => {
                                            const type = inferVariableType(varName);
                                            const value = parsedVariables[varName] ?? '';
                                            return (
                                                <div key={varName} className="flex flex-col gap-1.5">
                                                    <Label className="text-xs font-medium text-foreground capitalize">
                                                        {varName.replace(/_/g, ' ')}
                                                    </Label>
                                                    {type === 'image' ? (
                                                        <div className="flex flex-col gap-1">
                                                            <Input
                                                                type="url"
                                                                value={value}
                                                                onChange={(e) =>
                                                                    onVariableEdit(slidePreview.templateId, varName, e.target.value)
                                                                }
                                                                placeholder="https://..."
                                                                className="h-7 text-xs"
                                                            />
                                                            {value && (
                                                                <img
                                                                    src={value}
                                                                    alt={varName}
                                                                    className="h-14 w-auto rounded border border-border object-contain"
                                                                    onError={(e) => {
                                                                        (e.target as HTMLImageElement).style.display = 'none';
                                                                    }}
                                                                />
                                                            )}
                                                        </div>
                                                    ) : type === 'url' ? (
                                                        <Input
                                                            type="url"
                                                            value={value}
                                                            onChange={(e) =>
                                                                onVariableEdit(slidePreview.templateId, varName, e.target.value)
                                                            }
                                                            placeholder="https://..."
                                                            className="h-7 text-xs"
                                                        />
                                                    ) : (
                                                        <Input
                                                            value={value}
                                                            onChange={(e) =>
                                                                onVariableEdit(slidePreview.templateId, varName, e.target.value)
                                                            }
                                                            placeholder={varName}
                                                            className="h-7 text-xs"
                                                        />
                                                    )}
                                                </div>
                                            );
                                        })}
                                    </div>
                                </div>
                            </>
                        )}

                        {/* Preview area */}
                        <div className="flex-1 min-h-0 min-w-0">
                            {currentPreview?.rendered ? (
                                slidePreview.channel === 'email' ? (
                                    <iframe
                                        srcDoc={injectEditableSupport(currentPreview.rendered)}
                                        sandbox="allow-scripts"
                                        className="w-full h-full border-0 bg-white"
                                        title="Editable Rendered Preview"
                                    />
                                ) : slidePreview.channel === 'whatsapp' ? (
                                    <WhatsAppMobilePreview rendered={currentPreview.rendered} mediaUrl={whatsappMediaUrl} />
                                ) : (
                                    <div className="h-full p-4 overflow-y-auto text-sm text-foreground whitespace-pre-wrap">
                                        {currentPreview.rendered}
                                    </div>
                                )
                            ) : (
                                <div className="flex items-center justify-center h-full text-muted-foreground italic">
                                    No rendered output yet. Click "Re-render" above.
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            ) : (
                <div className="flex items-center justify-center h-40 text-muted-foreground italic">
                    Preview is not available.
                </div>
            )}
        </SlidePanel>
    );
};

export default EditablePreviewPanel;
