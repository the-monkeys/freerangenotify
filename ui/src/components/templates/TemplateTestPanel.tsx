import React, { useState, useEffect } from 'react';
import { templatesAPI, usersAPI } from '../../services/api';
import type { Template, User } from '../../types';
import { useAuth } from '../../contexts/AuthContext';
import VerifyPhoneDialog from '../VerifyPhoneDialog';
import { extractErrorMessage } from '../../lib/utils';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Input } from '../ui/input';
import { Badge } from '../ui/badge';
import { Card, CardContent } from '../ui/card';
import { toast } from 'sonner';
import { ChevronsUpDown, Loader2 } from 'lucide-react';

interface TemplateTestPanelProps {
    apiKey: string;
    template: Template;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

const TemplateTestPanel: React.FC<TemplateTestPanelProps> = ({
    apiKey,
    template,
    open,
    onOpenChange,
}) => {
    const [users, setUsers] = useState<User[]>([]);
    const { user } = useAuth();
    const [isVerifyDialogOpen, setIsVerifyDialogOpen] = useState(false);
    const [selectedUserId, setSelectedUserId] = useState('');
    const [variablesJson, setVariablesJson] = useState('');
    const [jsonError, setJsonError] = useState('');
    const [preview, setPreview] = useState('');
    const [loadingPreview, setLoadingPreview] = useState(false);
    const [loadingSend, setLoadingSend] = useState(false);
    const [loadingUsers, setLoadingUsers] = useState(false);
    const [savingDefaults, setSavingDefaults] = useState(false);
    const [variableFilter, setVariableFilter] = useState('');
    const [showAllQuickVariables, setShowAllQuickVariables] = useState(false);

    const MAX_QUICK_VARIABLES = 18;

    const getStorageKey = () => `frn:template-test-variables:${template.app_id}:${template.id}`;

    // Initialize variables JSON from template.variables
    useEffect(() => {
        if (open) {
            let initialJson = '';
            try {
                initialJson = localStorage.getItem(getStorageKey()) || '';
            } catch {
                // Ignore storage failures.
            }

            if (!initialJson) {
                const sampleData = (template.metadata?.sample_data as Record<string, any> | undefined) || null;
                if (sampleData) {
                    initialJson = JSON.stringify(sampleData, null, 2);
                } else {
                    const defaultVars = (template.variables || []).reduce((acc, v) => {
                        acc[v] = '';
                        return acc;
                    }, {} as Record<string, string>);
                    initialJson = JSON.stringify(defaultVars, null, 2);
                }
            }

            setVariablesJson(initialJson);
            setPreview('');
            setJsonError('');
            setSelectedUserId('');
            setVariableFilter('');
            setShowAllQuickVariables(false);
            fetchUsers();
        }
    }, [open, template.id]);

    const fetchUsers = async () => {
        setLoadingUsers(true);
        try {
            const res = await usersAPI.list(apiKey, 1, 100);
            setUsers(res.users || []);
        } catch {
            // Silently handle — user can still type a user_id
        } finally {
            setLoadingUsers(false);
        }
    };

    const parseVariables = (): Record<string, any> | null => {
        try {
            const parsed = JSON.parse(variablesJson || '{}');
            setJsonError('');
            return parsed;
        } catch {
            setJsonError('Invalid JSON — check your syntax.');
            return null;
        }
    };

    const updateVariablesJson = (value: string) => {
        setVariablesJson(value);
        setJsonError('');
        try {
            localStorage.setItem(getStorageKey(), value);
        } catch {
            // Ignore storage failures.
        }
    };

    const handleSaveDefaults = async () => {
        const vars = parseVariables();
        if (!vars) return;

        setSavingDefaults(true);
        try {
            await templatesAPI.update(apiKey, template.id, {
                metadata: {
                    ...(template.metadata || {}),
                    sample_data: vars,
                },
            });
            try {
                localStorage.setItem(getStorageKey(), JSON.stringify(vars, null, 2));
            } catch {
                // Ignore storage failures.
            }
            toast.success('Saved as template defaults');
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to save defaults'));
        } finally {
            setSavingDefaults(false);
        }
    };

    const handlePreview = async () => {
        const vars = parseVariables();
        if (!vars) return;

        try {
            localStorage.setItem(getStorageKey(), JSON.stringify(vars, null, 2));
        } catch {
            // Ignore storage failures.
        }

        setLoadingPreview(true);
        try {
            const result = await templatesAPI.render(apiKey, template.id, { data: vars });
            setPreview(result.rendered_body || JSON.stringify(result));
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Preview failed'));
        } finally {
            setLoadingPreview(false);
        }
    };

    const handleSendTest = async () => {
        if (template.channel === 'whatsapp' && !user?.phone_verified) {
            setIsVerifyDialogOpen(true);
            return;
        }

        if (!selectedUserId) {
            toast.error('Select a recipient user');
            return;
        }
        const selectedUser = users.find(u => u.user_id === selectedUserId);
        if (!selectedUser?.email) {
            toast.error('Selected user does not have an email address');
            return;
        }
        const vars = parseVariables();
        if (!vars) return;

        try {
            localStorage.setItem(getStorageKey(), JSON.stringify(vars, null, 2));
        } catch {
            // Ignore storage failures.
        }

        setLoadingSend(true);
        try {
            await templatesAPI.sendTest(apiKey, template.id, {
                to_email: selectedUser.email,
                sample_data: vars,
            });
            toast.success(`Test notification sent to ${selectedUser.email}`);
        } catch (err: any) {
            toast.error(extractErrorMessage(err, 'Failed to send test'));
        } finally {
            setLoadingSend(false);
        }
    };

    const variableHint = template.variables && template.variables.length > 0
        ? `This template expects: ${template.variables.join(', ')}`
        : 'This template has no declared variables.';

    return (
        <SlidePanel
            open={open}
            onClose={() => onOpenChange(false)}
            title="Send Test Notification"
        >
            <div className="space-y-4 p-1">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-medium">{template.name}</span>
                    <Badge variant="outline" className="text-xs">{template.channel}</Badge>
                    <Badge variant="secondary" className="text-xs">
                        {(template.variables || []).length} variables
                    </Badge>
                </div>

                <Card className="border-border/80">
                    <CardContent className="space-y-1.5 p-3.5">
                        <Label className="text-xs">Recipient User</Label>
                        <Select value={selectedUserId} onValueChange={setSelectedUserId}>
                            <SelectTrigger>
                                <SelectValue placeholder={loadingUsers ? 'Loading users...' : 'Select a user...'} />
                            </SelectTrigger>
                            <SelectContent>
                                {users.map(u => (
                                    <SelectItem key={u.user_id} value={u.user_id}>
                                        {u.email || u.user_id}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </CardContent>
                </Card>

                <Card className="border-border/80">
                    <CardContent className="space-y-2 p-3.5">
                        <Label className="text-xs">Variables (JSON)</Label>
                        <Textarea
                            className="h-44 font-mono text-xs"
                            value={variablesJson}
                            onChange={(e) => updateVariablesJson(e.target.value)}
                            placeholder='{"user_name": "Alice"}'
                        />
                        {jsonError && <p className="text-xs text-red-600">{jsonError}</p>}
                        <p className="text-xs text-muted-foreground">{variableHint}</p>

                        {template.variables && template.variables.length > 0 && (() => {
                        try {
                            const parsed = JSON.parse(variablesJson || '{}') as Record<string, any>;
                            const filtered = template.variables.filter((v) =>
                                v.toLowerCase().includes(variableFilter.trim().toLowerCase())
                            );
                            const filteredVariables = showAllQuickVariables
                                ? filtered
                                : filtered.slice(0, MAX_QUICK_VARIABLES);
                            return (
                                <div className="rounded-lg border border-border/70 bg-muted/30 p-3">
                                    <div className="mb-2 flex items-center justify-between gap-2">
                                        <p className="text-xs font-semibold text-muted-foreground">Quick Variable Form</p>
                                        <p className="text-xs text-muted-foreground">{filtered.length} match</p>
                                    </div>
                                    <div className="mb-2">
                                        <Input
                                            value={variableFilter}
                                            onChange={(e) => setVariableFilter(e.target.value)}
                                            placeholder="Search variable name..."
                                        />
                                    </div>
                                    <div className="grid max-h-60 grid-cols-1 gap-2 overflow-y-auto pr-1 md:grid-cols-2">
                                        {filteredVariables.map((variable) => (
                                            <div key={variable} className="space-y-1">
                                                <Label className="text-xs">{variable}</Label>
                                                <Input
                                                    value={parsed?.[variable] == null ? '' : String(parsed[variable])}
                                                    onChange={(e) => {
                                                        const next = { ...parsed, [variable]: e.target.value };
                                                        updateVariablesJson(JSON.stringify(next, null, 2));
                                                    }}
                                                    placeholder={`Value for ${variable}`}
                                                />
                                            </div>
                                        ))}
                                    </div>
                                    {filtered.length > MAX_QUICK_VARIABLES && (
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="sm"
                                            className="mt-2 h-6 px-2 text-xs"
                                            onClick={() => setShowAllQuickVariables((prev) => !prev)}
                                        >
                                            <ChevronsUpDown className="h-3.5 w-3.5" />
                                            {showAllQuickVariables
                                                ? 'Show fewer'
                                                : `Show all ${filtered.length}`}
                                        </Button>
                                    )}
                                    {filtered.length === 0 && (
                                        <p className="text-xs text-muted-foreground mt-2">No variables match your search.</p>
                                    )}
                                </div>
                            );
                        } catch {
                            return (
                                <p className="text-xs text-amber-600">
                                    Quick Variable Form is unavailable because JSON is invalid.
                                </p>
                            );
                        }
                    })()}
                    </CardContent>
                </Card>

                {/* Action buttons */}
                <div className="flex flex-wrap gap-2">
                    <Button
                        variant="outline"
                        onClick={handlePreview}
                        disabled={loadingPreview}
                        className="flex-1 min-w-30"
                    >
                        {loadingPreview ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Preview
                    </Button>
                    <Button
                        variant="outline"
                        onClick={handleSaveDefaults}
                        disabled={savingDefaults}
                        className="flex-1 min-w-34"
                    >
                        {savingDefaults ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Save as Default
                    </Button>
                    <Button
                        onClick={handleSendTest}
                        disabled={loadingSend || !selectedUserId}
                        className="flex-1 min-w-30"
                    >
                        {loadingSend ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Send Test
                    </Button>
                </div>

                {/* Preview output */}
                {preview && (
                    <div className="space-y-1.5 rounded-lg border border-border/80 bg-background p-3">
                        <Label className="text-xs">Preview Output</Label>
                        {template.channel === 'email' ? (
                            <iframe
                                srcDoc={preview}
                                sandbox=""
                                className="w-full border rounded-md bg-white"
                                style={{ height: '300px' }}
                                title="Template preview"
                            />
                        ) : (
                            <div className="bg-muted/35 p-4 rounded-md border border-border overflow-y-auto max-h-75 text-sm text-foreground whitespace-pre-wrap">
                                {preview}
                            </div>
                        )}
                    </div>
                )}
            </div>
            <VerifyPhoneDialog 
                open={isVerifyDialogOpen} 
                onOpenChange={setIsVerifyDialogOpen} 
            />
        </SlidePanel>
    );
};

export default TemplateTestPanel;
