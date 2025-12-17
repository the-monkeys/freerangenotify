import React, { useEffect, useState } from 'react';
import { templatesAPI } from '../services/api';
import type { Template, CreateTemplateRequest } from '../types';

interface AppTemplatesProps {
    appId: string;
    apiKey: string;
}

const AppTemplates: React.FC<AppTemplatesProps> = ({ appId, apiKey }) => {
    const [templates, setTemplates] = useState<Template[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddForm, setShowAddForm] = useState(false);
    const [formData, setFormData] = useState<CreateTemplateRequest>({
        app_id: appId,
        name: '',
        channel: 'email',
        subject: '',
        body: '',
        description: '',
        variables: []
    });

    const [varInput, setVarInput] = useState('');

    // Preview state
    const [activePreviews, setActivePreviews] = useState<Record<string, { data: string, rendered: string, loading: boolean }>>({});

    useEffect(() => {
        if (apiKey) {
            fetchTemplates();
        }
    }, [apiKey]);

    const fetchTemplates = async () => {
        setLoading(true);
        try {
            const data = await templatesAPI.list(apiKey);
            setTemplates(data || []);
        } catch (error) {
            console.error('Failed to fetch templates:', error);
        } finally {
            setLoading(false);
        }
    };

    const handleCreateTemplate = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            await templatesAPI.create(apiKey, { ...formData, app_id: appId });
            setShowAddForm(false);
            setFormData({
                app_id: appId,
                name: '',
                channel: 'email',
                subject: '',
                body: '',
                description: '',
                variables: []
            });
            fetchTemplates();
        } catch (error) {
            console.error('Failed to create template:', error);
            alert('Failed to create template: ' + error);
        }
    };

    const handleAddVariable = () => {
        if (varInput && formData.variables && !formData.variables.includes(varInput)) {
            setFormData({ ...formData, variables: [...formData.variables, varInput] });
            setVarInput('');
        }
    };

    const handleDeleteTemplate = async (id: string) => {
        if (!window.confirm('Delete this template?')) return;
        try {
            await templatesAPI.delete(apiKey, id);
            fetchTemplates();
        } catch (error) {
            console.error('Failed to delete template:', error);
        }
    };

    const togglePreview = (tmplId: string) => {
        if (activePreviews[tmplId]) {
            const newPreviews = { ...activePreviews };
            delete newPreviews[tmplId];
            setActivePreviews(newPreviews);
        } else {
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { data: '{}', rendered: '', loading: false }
            });
        }
    };

    const handleRenderPreview = async (tmplId: string) => {
        const preview = activePreviews[tmplId];
        if (!preview) return;

        let parsedData = {};
        try {
            parsedData = JSON.parse(preview.data);
        } catch (e) {
            alert('Invalid JSON data');
            return;
        }

        setActivePreviews({
            ...activePreviews,
            [tmplId]: { ...preview, loading: true }
        });

        try {
            const resp = await templatesAPI.render(apiKey, tmplId, { data: parsedData });
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { ...preview, rendered: resp.rendered_body, loading: false }
            });
        } catch (error) {
            console.error('Failed to render preview:', error);
            alert('Failed to render preview');
            setActivePreviews({
                ...activePreviews,
                [tmplId]: { ...preview, loading: false }
            });
        }
    };

    if (loading) return <div className="center">Loading templates...</div>;

    return (
        <div className="card">
            <div className="flex justify-between items-center mb-6">
                <h3 style={{ margin: 0 }}>Notification Templates</h3>
                <button
                    className="btn btn-primary"
                    onClick={() => setShowAddForm(!showAddForm)}
                >
                    {showAddForm ? 'Cancel' : 'Create Template'}
                </button>
            </div>

            {showAddForm && (
                <form onSubmit={handleCreateTemplate} className="mb-8" style={{ background: '#1a202c', padding: '1.5rem', borderRadius: '0.5rem' }}>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div className="form-group">
                            <label className="form-label">Template Name</label>
                            <input
                                type="text"
                                className="form-input"
                                value={formData.name}
                                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                                required
                                placeholder="e.g. welcome_email"
                            />
                        </div>
                        <div className="form-group">
                            <label className="form-label">Channel</label>
                            <select
                                className="form-input"
                                value={formData.channel}
                                onChange={(e) => setFormData({ ...formData, channel: e.target.value as any })}
                            >
                                <option value="email">Email</option>
                                <option value="push">Push</option>
                                <option value="sms">SMS</option>
                                <option value="webhook">Webhook</option>
                                <option value="in_app">In-App</option>
                            </select>
                        </div>
                    </div>
                    <div className="form-group">
                        <label className="form-label">Subject (for Email)</label>
                        <input
                            type="text"
                            className="form-input"
                            value={formData.subject || ''}
                            onChange={(e) => setFormData({ ...formData, subject: e.target.value })}
                            placeholder="Email subject"
                        />
                    </div>
                    <div className="form-group">
                        <label className="form-label">Body / Content</label>
                        <textarea
                            className="form-input"
                            style={{ minHeight: '150px', fontFamily: 'monospace' }}
                            value={formData.body}
                            onChange={(e) => {
                                // Simple regex to auto-detect variables like {{.var_name}}
                                const newBody = e.target.value;
                                const regex = /{{\s*\.?(\w+)\s*}}/g;
                                const matches = new Set<string>();
                                let match;
                                while ((match = regex.exec(newBody)) !== null) {
                                    if (match[1]) matches.add(match[1]);
                                }
                                // Combine custom added vars with auto-detected ones
                                const currentVars = new Set(formData.variables || []);
                                for (const m of matches) currentVars.add(m);

                                setFormData({
                                    ...formData,
                                    body: newBody,
                                    variables: Array.from(currentVars)
                                });
                            }}
                            required
                            placeholder="Hello {{.name}}, welcome!"
                        />
                        <p style={{ fontSize: '0.75rem', color: '#a0aec0', marginTop: '0.25rem' }}>
                            Use <code>{'{{.variable_name}}'}</code> syntax. Detected variables will enter the list below automatically.
                        </p>
                    </div>
                    <div className="form-group">
                        <label className="form-label">Variables (Must be declared to pass validation)</label>
                        <div className="flex gap-2">
                            <input
                                type="text"
                                className="form-input"
                                value={varInput}
                                onChange={(e) => setVarInput(e.target.value)}
                                placeholder="name"
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter') {
                                        e.preventDefault();
                                        handleAddVariable();
                                    }
                                }}
                            />
                            <button type="button" className="btn btn-secondary" onClick={handleAddVariable}>Add</button>
                        </div>
                        <div className="mt-2 flex gap-2 flex-wrap">
                            {(formData.variables || []).map(v => (
                                <span key={v} style={{ background: '#2d3748', padding: '0.25rem 0.75rem', borderRadius: '1rem', fontSize: '0.875rem' }}>
                                    {v}
                                    <button
                                        type="button"
                                        onClick={() => setFormData({ ...formData, variables: formData.variables?.filter(x => x !== v) || [] })}
                                        style={{ marginLeft: '0.5rem', background: 'none', border: 'none', color: '#f56565', cursor: 'pointer' }}
                                    >
                                        &times;
                                    </button>
                                </span>
                            ))}
                        </div>
                    </div>
                    <div className="flex justify-end mt-4">
                        <button type="submit" className="btn btn-primary">Create Template</button>
                    </div>
                </form>
            )}

            {!templates || templates.length === 0 ? (
                <p style={{ color: '#718096', textAlign: 'center', padding: '2rem' }}>No templates found.</p>
            ) : (
                <div className="grid grid-cols-1 gap-6">
                    {templates.map((tmpl) => (
                        <div key={tmpl.id} className="card" style={{ background: '#1a202c', border: '1px solid #2d3748' }}>
                            <div className="flex justify-between items-start mb-4">
                                <div>
                                    <h4 style={{ margin: 0, color: '#667eea', fontSize: '1.25rem' }}>{tmpl.name}</h4>
                                    <p style={{ fontSize: '0.875rem', color: '#a0aec0', margin: '0.25rem 0' }}>{tmpl.description || 'No description'}</p>
                                </div>
                                <span style={{ fontSize: '0.75rem', background: '#2d3748', padding: '0.25rem 0.75rem', borderRadius: '1rem', textTransform: 'uppercase' }}>
                                    {tmpl.channel}
                                </span>
                            </div>

                            <div className="mb-4" style={{ background: '#2d374820', padding: '1rem', borderRadius: '0.5rem', border: '1px dashed #2d3748' }}>
                                <div style={{ fontSize: '0.75rem', color: '#718096', marginBottom: '0.5rem' }}>TEMPLATE BODY</div>
                                <pre style={{ margin: 0, whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: '0.875rem' }}>{tmpl.body}</pre>
                            </div>

                            <div className="flex justify-between items-center mb-4">
                                <div style={{ fontSize: '0.75rem' }}>
                                    <strong>Variables:</strong> {tmpl.variables && tmpl.variables.length > 0 ? tmpl.variables.join(', ') : 'None'}
                                </div>
                                <div className="flex gap-2">
                                    <button
                                        onClick={() => togglePreview(tmpl.id)}
                                        className="btn btn-secondary"
                                        style={{ padding: '0.25rem 0.75rem', fontSize: '0.75rem' }}
                                    >
                                        {activePreviews[tmpl.id] ? 'Close Preview' : 'Preview'}
                                    </button>
                                    <button
                                        onClick={() => handleDeleteTemplate(tmpl.id)}
                                        className="btn btn-danger"
                                        style={{ padding: '0.25rem 0.75rem', fontSize: '0.75rem' }}
                                    >
                                        Delete
                                    </button>
                                </div>
                            </div>

                            {activePreviews[tmpl.id] && (
                                <div style={{ marginTop: '1rem', borderTop: '1px solid #2d3748', paddingTop: '1rem' }}>
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                        <div>
                                            <div style={{ fontSize: '0.75rem', color: '#718096', marginBottom: '0.5rem' }}>PREVIEW DATA (JSON)</div>
                                            <textarea
                                                className="form-input"
                                                style={{ height: '100px', fontFamily: 'monospace', fontSize: '0.75rem' }}
                                                value={activePreviews[tmpl.id].data}
                                                onChange={(e) => setActivePreviews({
                                                    ...activePreviews,
                                                    [tmpl.id]: { ...activePreviews[tmpl.id], data: e.target.value }
                                                })}
                                                placeholder='{"name": "Jack"}'
                                            />
                                            <button
                                                className="btn btn-primary mt-2"
                                                style={{ width: '100%', fontSize: '0.75rem' }}
                                                onClick={() => handleRenderPreview(tmpl.id)}
                                                disabled={activePreviews[tmpl.id].loading}
                                            >
                                                {activePreviews[tmpl.id].loading ? 'Rendering...' : 'Render Preview'}
                                            </button>
                                        </div>
                                        <div>
                                            <div style={{ fontSize: '0.75rem', color: '#718096', marginBottom: '0.5rem' }}>RENDERED OUTPUT</div>
                                            <div style={{
                                                background: '#000',
                                                height: '100px',
                                                padding: '0.5rem',
                                                borderRadius: '0.25rem',
                                                overflowY: 'auto',
                                                fontSize: '0.875rem',
                                                color: '#4ade80'
                                            }}>
                                                {activePreviews[tmpl.id].rendered || <span style={{ color: '#4a5568' }}>Click Render to see output...</span>}
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
};

export default AppTemplates;
