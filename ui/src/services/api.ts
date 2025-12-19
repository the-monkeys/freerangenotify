import axios from 'axios';
import { Application } from '../types';

const API_BASE_URL = 'http://localhost:5000/api/v1/apps';

export const fetchApplications = async (): Promise<Application[]> => {
    const response = await axios.get(API_BASE_URL);
    return response.data;
};

export const fetchApplicationById = async (id: string): Promise<Application> => {
    const response = await axios.get(`${API_BASE_URL}/${id}`);
    return response.data;
};

export const createApplication = async (application: Application): Promise<Application> => {
    const response = await axios.post(API_BASE_URL, application);
    return response.data;
};

export const updateApplication = async (id: string, application: Application): Promise<Application> => {
    const response = await axios.put(`${API_BASE_URL}/${id}`, application);
    return response.data;
};

export const deleteApplication = async (id: string): Promise<void> => {
    await axios.delete(`${API_BASE_URL}/${id}`);
};

export const regenerateAPIKey = async (id: string): Promise<Application> => {
    const response = await axios.post(`${API_BASE_URL}/${id}/regenerate-key`);
    return response.data;
};

export const updateApplicationSettings = async (id: string, settings: any): Promise<Application> => {
    const response = await axios.put(`${API_BASE_URL}/${id}/settings`, settings);
    return response.data;
};

export const fetchApplicationSettings = async (id: string): Promise<any> => {
    const response = await axios.get(`${API_BASE_URL}/${id}/settings`);
    return response.data;
};