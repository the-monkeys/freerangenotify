import { HttpClient } from './client';
import type { WorkflowBuilder } from './workflow_builder';
import type {
    CreateWorkflowParams,
    UpdateWorkflowParams,
    TriggerWorkflowParams,
    Workflow,
    WorkflowExecution,
    WorkflowListResponse,
    ExecutionListResponse,
} from './types';

export class WorkflowsClient {
    constructor(private readonly http: HttpClient) { }

    async create(params: CreateWorkflowParams): Promise<Workflow> {
        return this.http.request<Workflow>('POST', '/workflows/', params);
    }

    async get(workflowId: string): Promise<Workflow> {
        return this.http.request<Workflow>('GET', `/workflows/${workflowId}`);
    }

    async update(workflowId: string, params: UpdateWorkflowParams): Promise<Workflow> {
        return this.http.request<Workflow>('PUT', `/workflows/${workflowId}`, params);
    }

    async delete(workflowId: string): Promise<void> {
        await this.http.request('DELETE', `/workflows/${workflowId}`);
    }

    async list(page?: number, pageSize?: number): Promise<WorkflowListResponse> {
        const query: Record<string, string | undefined> = {};
        if (page) query.page = String(page);
        if (pageSize) query.page_size = String(pageSize);
        return this.http.request<WorkflowListResponse>('GET', '/workflows/', undefined, query);
    }

    async trigger(params: TriggerWorkflowParams): Promise<WorkflowExecution> {
        return this.http.request<WorkflowExecution>('POST', '/workflows/trigger', params);
    }

    async getExecution(executionId: string): Promise<WorkflowExecution> {
        return this.http.request<WorkflowExecution>(
            'GET',
            `/workflows/executions/${executionId}`,
        );
    }

    async listExecutions(
        page?: number,
        pageSize?: number,
    ): Promise<ExecutionListResponse> {
        const query: Record<string, string | undefined> = {};
        if (page) query.page = String(page);
        if (pageSize) query.page_size = String(pageSize);
        return this.http.request<ExecutionListResponse>(
            'GET',
            '/workflows/executions',
            undefined,
            query,
        );
    }

    async cancelExecution(executionId: string): Promise<void> {
        await this.http.request('POST', `/workflows/executions/${executionId}/cancel`);
    }

    // ── Builder Integration ──

    /** Create a workflow from a WorkflowBuilder. */
    async createFromBuilder(builder: WorkflowBuilder): Promise<Workflow> {
        const params = builder.build();
        return this.create(params);
    }

    /** Update a workflow from a WorkflowBuilder. */
    async updateFromBuilder(id: string, builder: WorkflowBuilder): Promise<Workflow> {
        const params = builder.build();
        return this.update(id, {
            name: params.name,
            description: params.description,
            steps: params.steps,
        });
    }
}
