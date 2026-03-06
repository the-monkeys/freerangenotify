import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { workflowsAPI, usersAPI, applicationsAPI } from '../../services/api';
import type { Workflow, WorkflowStep, WorkflowStatus, User, Application } from '../../types';
import WorkflowStepCard from '../../components/workflows/WorkflowStepCard';
import WorkflowStepEditor from '../../components/workflows/WorkflowStepEditor';
import ResourcePicker from '../../components/ResourcePicker';
import JsonEditor from '../../components/JsonEditor';
import { SlidePanel } from '../../components/ui/slide-panel';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Textarea } from '../../components/ui/textarea';
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card';
import { ArrowLeft, Plus, ChevronDown, Loader2, Play, Save } from 'lucide-react';
import { toast } from 'sonner';

interface BuilderStep {
    name: string;
    type: WorkflowStep['type'];
    order: number;
    config: WorkflowStep['config'];
    on_success?: string;
    on_failure?: string;
    skip_if?: WorkflowStep['skip_if'];
}

const WorkflowBuilder: React.FC = () => {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const isEditMode = !!id;

    // App context
    const [apiKey, setApiKey] = useState<string | null>(
        localStorage.getItem('last_api_key')
    );
    const [selectedAppId, setSelectedAppId] = useState<string | null>(
        localStorage.getItem('last_app_id')
    );

    // Form state
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [triggerId, setTriggerId] = useState('');
    const [steps, setSteps] = useState<BuilderStep[]>([]);
    const [status, setStatus] = useState<WorkflowStatus>('draft');

    // Step editor
    const [showStepEditor, setShowStepEditor] = useState(false);
    const [editingStepIndex, setEditingStepIndex] = useState<number | null>(null);

    // Save / trigger state
    const [saving, setSaving] = useState(false);
    const [loadingWorkflow, setLoadingWorkflow] = useState(false);

    // Test trigger state
    const [showTestTrigger, setShowTestTrigger] = useState(false);
    const [testUserId, setTestUserId] = useState<string | null>(null);
    const [testPayload, setTestPayload] = useState('{}');
    const [triggering, setTriggering] = useState(false);

    // Load existing workflow in edit mode
    useEffect(() => {
        if (isEditMode && apiKey && id) {
            setLoadingWorkflow(true);
            workflowsAPI.get(apiKey, id)
                .then((w: Workflow) => {
                    setName(w.name);
                    setDescription(w.description || '');
                    setTriggerId(w.trigger_id);
                    setStatus(w.status);
                    setSteps(w.steps.map(({ id: _id, ...rest }) => rest));
                })
                .catch(() => toast.error('Failed to load workflow'))
                .finally(() => setLoadingWorkflow(false));
        }
    }, [isEditMode, apiKey, id]);

    const handleAppSelect = async (appId: string | null) => {
        if (!appId) return;
        try {
            const apps = await applicationsAPI.list();
            const app = apps.find((a: Application) => a.app_id === appId);
            if (app) {
                setSelectedAppId(app.app_id);
                setApiKey(app.api_key);
                localStorage.setItem('last_app_id', app.app_id);
                localStorage.setItem('last_api_key', app.api_key);
            }
        } catch {
            toast.error('Failed to load application');
        }
    };

    // Step management
    const addStep = (step: BuilderStep) => {
        setSteps(prev => [...prev, { ...step, order: prev.length + 1 }]);
        setShowStepEditor(false);
    };

    const updateStep = (index: number, step: BuilderStep) => {
        setSteps(prev =>
            prev.map((s, i) => (i === index ? { ...step, order: i + 1 } : s))
        );
        setEditingStepIndex(null);
        setShowStepEditor(false);
    };

    const removeStep = (index: number) => {
        setSteps(prev =>
            prev.filter((_, i) => i !== index).map((s, i) => ({ ...s, order: i + 1 }))
        );
    };

    // Save
    const handleSave = async (targetStatus?: WorkflowStatus) => {
        if (!apiKey) {
            toast.error('Select an application first');
            return;
        }
        if (!name.trim() || !triggerId.trim()) {
            toast.error('Name and Trigger ID are required');
            return;
        }
        if (steps.length === 0) {
            toast.error('Add at least one step');
            return;
        }

        setSaving(true);
        try {
            const payload = {
                name: name.trim(),
                description: description.trim(),
                trigger_id: triggerId.trim(),
                steps,
                status: targetStatus || status,
            };

            if (isEditMode && id) {
                await workflowsAPI.update(apiKey, id, payload);
                if (targetStatus) setStatus(targetStatus);
                toast.success(`Workflow ${targetStatus === 'active' ? 'activated' : 'saved'}`);
            } else {
                const created = await workflowsAPI.create(apiKey, payload);
                toast.success('Workflow created');
                navigate(`/workflows/${created.id}`, { replace: true });
            }
        } catch {
            toast.error('Failed to save workflow');
        } finally {
            setSaving(false);
        }
    };

    // Test trigger
    const handleTrigger = async () => {
        if (!apiKey || !testUserId) return;
        setTriggering(true);
        try {
            let payload: Record<string, any> = {};
            if (testPayload.trim()) {
                payload = JSON.parse(testPayload);
            }
            await workflowsAPI.trigger(apiKey, {
                trigger_id: triggerId,
                user_id: testUserId,
                payload,
            });
            toast.success('Workflow triggered successfully');
        } catch (err: any) {
            toast.error(err?.response?.data?.error || 'Failed to trigger workflow');
        } finally {
            setTriggering(false);
        }
    };

    if (!apiKey) {
        return (
            <div className="p-6 max-w-4xl mx-auto space-y-6">
                <h1 className="text-2xl font-semibold text-foreground">
                    {isEditMode ? 'Edit Workflow' : 'New Workflow'}
                </h1>
                <div className="max-w-xs">
                    <ResourcePicker<Application>
                        label="Application"
                        value={selectedAppId}
                        onChange={handleAppSelect}
                        fetcher={async () => applicationsAPI.list()}
                        labelKey="app_name"
                        valueKey="app_id"
                        placeholder="Select an application..."
                        hint="Select an app to create workflows for"
                    />
                </div>
            </div>
        );
    }

    if (loadingWorkflow) {
        return (
            <div className="flex items-center justify-center h-64">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
        );
    }

    return (
        <div className="p-6 max-w-4xl mx-auto space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                    <Button variant="ghost" size="sm" onClick={() => navigate('/workflows')}>
                        <ArrowLeft className="h-4 w-4 mr-1" />
                        Back
                    </Button>
                    <h1 className="text-xl font-semibold text-foreground">
                        {isEditMode ? 'Edit Workflow' : 'New Workflow'}
                    </h1>
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="outline"
                        onClick={() => handleSave()}
                        disabled={saving}
                    >
                        {saving ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : <Save className="h-4 w-4 mr-2" />}
                        Save Draft
                    </Button>
                    <Button
                        onClick={() => handleSave('active')}
                        disabled={saving}
                    >
                        {saving ? <Loader2 className="h-4 w-4 mr-2 animate-spin" /> : null}
                        Activate
                    </Button>
                </div>
            </div>

            {/* Basic Info */}
            <Card>
                <CardContent className="pt-6 space-y-4">
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label>Name <span className="text-destructive">*</span></Label>
                            <Input
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="e.g., Welcome Onboarding Flow"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label>Trigger ID <span className="text-destructive">*</span></Label>
                            <Input
                                value={triggerId}
                                onChange={(e) => setTriggerId(e.target.value)}
                                placeholder="e.g., user_signup"
                                className="font-mono"
                            />
                            <p className="text-xs text-muted-foreground">
                                Unique identifier used to trigger this workflow via API
                            </p>
                        </div>
                    </div>
                    <div className="space-y-2">
                        <Label>Description</Label>
                        <Textarea
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            placeholder="Describe what this workflow does..."
                            rows={2}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Steps */}
            <div>
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-medium text-foreground">Steps</h2>
                    <span className="text-sm text-muted-foreground">{steps.length} step{steps.length !== 1 ? 's' : ''}</span>
                </div>

                {steps.length === 0 && (
                    <div className="border-2 border-dashed border-border rounded-lg p-8 text-center">
                        <p className="text-sm text-muted-foreground mb-3">
                            No steps yet. Add your first step to define the notification flow.
                        </p>
                        <Button
                            variant="outline"
                            onClick={() => {
                                setEditingStepIndex(null);
                                setShowStepEditor(true);
                            }}
                        >
                            <Plus className="h-4 w-4 mr-2" />
                            Add First Step
                        </Button>
                    </div>
                )}

                {steps.length > 0 && (
                    <div className="space-y-0">
                        {steps.map((step, index) => (
                            <React.Fragment key={index}>
                                <WorkflowStepCard
                                    step={{ ...step, id: `step-${index}` }}
                                    index={index}
                                    onEdit={() => {
                                        setEditingStepIndex(index);
                                        setShowStepEditor(true);
                                    }}
                                    onRemove={() => removeStep(index)}
                                />
                                {/* Connector */}
                                {index < steps.length - 1 && (
                                    <div className="flex flex-col items-center py-1">
                                        <div className="w-px h-4 bg-border" />
                                        <ChevronDown className="h-3.5 w-3.5 text-muted-foreground -mt-0.5" />
                                    </div>
                                )}
                            </React.Fragment>
                        ))}

                        {/* Add step button */}
                        <div className="flex flex-col items-center pt-2">
                            <div className="w-px h-4 bg-border" />
                            <Button
                                variant="outline"
                                size="sm"
                                className="mt-1"
                                onClick={() => {
                                    setEditingStepIndex(null);
                                    setShowStepEditor(true);
                                }}
                            >
                                <Plus className="h-3.5 w-3.5 mr-1.5" />
                                Add Step
                            </Button>
                        </div>
                    </div>
                )}
            </div>

            {/* Test Trigger (collapsible) */}
            {isEditMode && status === 'active' && (
                <Card>
                    <CardHeader
                        className="cursor-pointer"
                        onClick={() => setShowTestTrigger(!showTestTrigger)}
                    >
                        <CardTitle className="flex items-center gap-2 text-base">
                            <Play className="h-4 w-4" />
                            Test Trigger
                            <ChevronDown className={`h-4 w-4 ml-auto transition-transform ${showTestTrigger ? 'rotate-180' : ''}`} />
                        </CardTitle>
                    </CardHeader>
                    {showTestTrigger && (
                        <CardContent className="space-y-4 pt-0">
                            <ResourcePicker<User>
                                label="User"
                                value={testUserId}
                                onChange={setTestUserId}
                                fetcher={async () => {
                                    const res = await usersAPI.list(apiKey!, 1, 100);
                                    return res.users || [];
                                }}
                                labelKey="email"
                                valueKey="user_id"
                                hint="Select the user who will receive this test notification"
                                placeholder="Search users..."
                                required
                            />
                            <JsonEditor
                                label="Payload"
                                value={testPayload}
                                onChange={setTestPayload}
                                hint='Variables for your template — e.g. {"user_name": "Alice"}'
                                rows={4}
                            />
                            <Button
                                onClick={handleTrigger}
                                disabled={!testUserId || triggering}
                            >
                                {triggering && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                                Trigger Workflow
                            </Button>
                        </CardContent>
                    )}
                </Card>
            )}

            {/* Step Editor Panel */}
            <SlidePanel
                open={showStepEditor}
                onClose={() => {
                    setShowStepEditor(false);
                    setEditingStepIndex(null);
                }}
                title={editingStepIndex !== null ? `Edit Step ${editingStepIndex + 1}` : 'Add Step'}
            >
                <WorkflowStepEditor
                    step={editingStepIndex !== null ? steps[editingStepIndex] : null}
                    apiKey={apiKey!}
                    onSave={(step) => {
                        if (editingStepIndex !== null) {
                            updateStep(editingStepIndex, step);
                        } else {
                            addStep(step);
                        }
                    }}
                    onCancel={() => {
                        setShowStepEditor(false);
                        setEditingStepIndex(null);
                    }}
                />
            </SlidePanel>
        </div>
    );
};

export default WorkflowBuilder;
