import React, { useState } from 'react';

const AppForm: React.FC<{ onSubmit: (data: any) => void; initialData?: any }> = ({ onSubmit, initialData }) => {
    const [formData, setFormData] = useState(initialData || { app_name: '', description: '' });

    const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
        const { name, value } = e.target;
        setFormData({ ...formData, [name]: value });
    };

    const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        onSubmit(formData);
    };

    return (
        <form onSubmit={handleSubmit} className="card">
            <h3 className="mb-4">{initialData ? 'Edit Application' : 'Create Application'}</h3>
            <div className="form-group">
                <label className="form-label" htmlFor="app_name">Application Name</label>
                <input
                    type="text"
                    id="app_name"
                    name="app_name"
                    className="form-input"
                    value={formData.app_name}
                    onChange={handleChange}
                    required
                />
            </div>
            <div className="form-group">
                <label className="form-label" htmlFor="description">Description</label>
                <textarea
                    id="description"
                    name="description"
                    className="form-input" // utilizing form-input class for consistency
                    value={formData.description}
                    onChange={handleChange}
                />
            </div>
            <button type="submit" className="btn btn-primary">Submit</button>
        </form>
    );
};

export default AppForm;