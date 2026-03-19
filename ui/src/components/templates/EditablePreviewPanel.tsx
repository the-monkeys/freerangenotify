import React, { useEffect, useCallback } from 'react';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Separator } from '../ui/separator';
import type { Template } from '../../types';

interface EditablePreviewPanelProps {
    slidePreview: { templateId: string; templateName: string; channel: string } | null;
    templates: Template[];
    activePreviews: Record<string, { data: string; rendered: string; loading: boolean }>;
    savingDefaults: Record<string, boolean>;
    onClose: () => void;
    onRenderPreview: (templateId: string) => void;
    onSaveDefaults: (template: Template) => void;
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

const EditablePreviewPanel: React.FC<EditablePreviewPanelProps> = ({
    slidePreview,
    templates,
    activePreviews,
    savingDefaults,
    onClose,
    onRenderPreview,
    onSaveDefaults,
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

    const variables = currentTemplate?.variables ?? [];
    const hasVariables = variables.length > 0;

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
                        {hasVariables && <span className="flex-1" />}
                        <Button
                            size="sm"
                            className="text-xs h-7"
                            onClick={() => onRenderPreview(slidePreview.templateId)}
                            disabled={!!currentPreview?.loading}
                        >
                            {currentPreview?.loading ? 'Rendering...' : 'Re-render'}
                        </Button>
                        {currentTemplate && (
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
                    </div>

                    {/* Body: optional variables sidebar + preview */}
                    <div className="flex flex-1 min-h-0 overflow-hidden">
                        {/* Variables sidebar */}
                        {hasVariables && (
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
