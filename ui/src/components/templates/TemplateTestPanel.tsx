import React, { useState, useEffect } from 'react';
import { templatesAPI, usersAPI } from '../../services/api';
import type { Template, User } from '../../types';
import { extractErrorMessage } from '../../lib/utils';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Badge } from '../ui/badge';
import { toast } from 'sonner';
import { Loader2 } from 'lucide-react';

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
    const [selectedUserId, setSelectedUserId] = useState('');
    const [variablesJson, setVariablesJson] = useState('');
    const [jsonError, setJsonError] = useState('');
    const [preview, setPreview] = useState('');
    const [loadingPreview, setLoadingPreview] = useState(false);
    const [loadingSend, setLoadingSend] = useState(false);
    const [loadingUsers, setLoadingUsers] = useState(false);

    // Initialize variables JSON from template.variables
    useEffect(() => {
        if (open) {
            const defaultVars = (template.variables || []).reduce((acc, v) => {
                acc[v] = '';
                return acc;
            }, {} as Record<string, string>);
            setVariablesJson(JSON.stringify(defaultVars, null, 2));
            setPreview('');
            setJsonError('');
            setSelectedUserId('');
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

    const handlePreview = async () => {
        const vars = parseVariables();
        if (!vars) return;

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
        if (!selectedUserId) {
            toast.error('Select a recipient user');
            return;
        }
        const vars = parseVariables();
        if (!vars) return;

        setLoadingSend(true);
        try {
            await templatesAPI.sendTest(apiKey, template.id, {
                user_id: selectedUserId,
                variables: vars,
            });
            const user = users.find(u => u.user_id === selectedUserId);
            toast.success(`Test notification sent to ${user?.email || selectedUserId}`);
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
            <div className="space-y-5 p-1">
                {/* Template info */}
                <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{template.name}</span>
                    <Badge variant="outline" className="text-xs">{template.channel}</Badge>
                </div>

                {/* User picker */}
                <div className="space-y-1.5">
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
                </div>

                {/* Variables editor */}
                <div className="space-y-1.5">
                    <Label className="text-xs">Variables (JSON)</Label>
                    <Textarea
                        className="h-[140px] font-mono text-xs"
                        value={variablesJson}
                        onChange={(e) => {
                            setVariablesJson(e.target.value);
                            setJsonError('');
                        }}
                        placeholder='{"user_name": "Alice"}'
                    />
                    {jsonError && <p className="text-xs text-red-600">{jsonError}</p>}
                    <p className="text-xs text-muted-foreground">{variableHint}</p>
                </div>

                {/* Action buttons */}
                <div className="flex gap-2">
                    <Button
                        variant="outline"
                        onClick={handlePreview}
                        disabled={loadingPreview}
                        className="flex-1"
                    >
                        {loadingPreview ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Preview
                    </Button>
                    <Button
                        onClick={handleSendTest}
                        disabled={loadingSend || !selectedUserId}
                        className="flex-1"
                    >
                        {loadingSend ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : null}
                        Send Test
                    </Button>
                </div>

                {/* Preview output */}
                {preview && (
                    <div className="space-y-1.5">
                        <Label className="text-xs">Preview Output</Label>
                        {template.channel === 'email' ? (
                            <iframe
                                srcDoc={preview}
                                sandbox=""
                                className="w-full border rounded bg-white"
                                style={{ height: '300px' }}
                                title="Template preview"
                            />
                        ) : (
                            <div className="bg-muted p-4 rounded border border-border overflow-y-auto max-h-[300px] text-sm text-foreground whitespace-pre-wrap">
                                {preview}
                            </div>
                        )}
                    </div>
                )}
            </div>
        </SlidePanel>
    );
};

export default TemplateTestPanel;
