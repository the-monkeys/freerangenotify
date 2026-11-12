import type { AxiosInstance } from 'axios';
import type { BillingLifecycle, BillingMerchant, BillingMerchantInput, BillingMode, BillingUpgradeReport, BillingConfigMap, BillingPage } from '../types/bizBilling';
import type {
  BizProduct,
  BizPlan,
  BizPlanAddon,
  BizConfig,
  BizSubscription,
  BizInvoice,
  BizPayment,
  CreateBizSubscriptionRequest,
  UpdateBizSubscriptionRequest,
  CreateBizInvoiceRequest,
  UpdateBizInvoiceRequest,
  RecordPaymentRequest,
  ChangeBizPlanRequest,
  RefundBizPaymentRequest,
  ApplyBizCouponRequest,
  ApplyBizCreditRequest,
  BizEstimate,
  CreateBizEstimateRequest,
  BizCoupon,
  CreateBizCouponRequest,
  BulkGenerateBizCouponsRequest,
  BizCreditNote,
  CreateBizCreditNoteRequest,
  BizRetainer,
  CreateBizRetainerRequest,
  BizContract,
  CreateBizContractRequest,
  AmendBizContractRequest,
  BizUsageMeter,
  CreateBizUsageMeterRequest,
  ReportBizUsageRequest,
  BizUsageSummary,
  BizExpense,
  CreateBizExpenseRequest,
  BizConnector,
  CreateBizConnectorRequest,
  UpdateBizConnectorRequest,
  BizStatement,
  BizPortalLink,
  BizListResponse,
} from '../types';

export function createBizBillingAPI(client: AxiosInstance, authHeaders: (apiKey: string) => Record<string,string>) {
type BizOne<T> = { success: boolean; data: T };

const bizOpts = (apiKey: string, key?: string) => ({ headers: { ...authHeaders(apiKey), ...(key ? {"Idempotency-Key": key} : {}) } });

return {
  readBilling: async <T>(apiKey:string,path:string,params:Record<string,unknown>={},signal?:AbortSignal):Promise<T> => (await client.get<BizOne<T>>(path,{...bizOpts(apiKey),params,signal})).data.data,
  listBilling: async <T>(apiKey:string,path:string,params:Record<string,unknown>={},signal?:AbortSignal):Promise<BizListResponse<T>> => (await client.get<BizListResponse<T>>(path,{...bizOpts(apiKey),params,signal})).data,
  submitBilling: async <T>(apiKey:string,path:string,payload:unknown,key:string,method:'POST'|'PUT'|'DELETE'='POST',signal?:AbortSignal):Promise<T> => (await client.request<BizOne<T>>({url:path,method,data:payload,...bizOpts(apiKey,key),signal})).data.data,
  getLifecycle: async (apiKey:string, signal?:AbortSignal) => (await client.get<BizOne<BillingLifecycle>>('/biz/lifecycle',{...bizOpts(apiKey),signal})).data.data,
  getBillingConfig: async <K extends keyof BillingConfigMap>(apiKey:string,kind:K,signal?:AbortSignal):Promise<BillingConfigMap[K]> => {
    const result=await client.get<BizOne<{data:BillingConfigMap[K]}>>(`/biz/config/${kind}`,{...bizOpts(apiKey),signal});
    if(!result.data.data?.data)throw new Error('Billing settings unavailable');
    return result.data.data.data;
  },
  saveBillingConfig: async <K extends keyof BillingConfigMap>(apiKey:string,kind:K,payload:Partial<BillingConfigMap[K]>,signal?:AbortSignal):Promise<BillingConfigMap[K]> => {
    const result=await client.put<BizOne<{data:BillingConfigMap[K]}>>(`/biz/config/${kind}`,payload,{...bizOpts(apiKey),signal});
    return result.data.data.data;
  },
  getMerchant: async (apiKey:string,mode:BillingMode,signal?:AbortSignal) => {
    const {data}=await client.get<BizOne<BillingMerchant>>(`/biz/gateway-connections/${mode}`,{...bizOpts(apiKey),signal});
    const m=data.data;
    return {app_id:m.app_id,configured:m.configured,mode:m.mode,revision:m.revision,account_id:m.account_id,connection_id:m.connection_id,readiness:m.readiness,api_authenticated:m.api_authenticated,account_verified:m.account_verified,automatic_ready:m.automatic_ready};
  },
  saveMerchant: async(apiKey:string,mode:BillingMode,payload:BillingMerchantInput,signal?:AbortSignal) => (await client.put<BizOne<BillingMerchant>>(`/biz/gateway-connections/${mode}`,payload,{...bizOpts(apiKey),signal})).data.data,
  disconnectMerchant: async(apiKey:string,mode:BillingMode,revision:number,signal?:AbortSignal) => (await client.delete<BizOne<BillingMerchant>>(`/biz/gateway-connections/${mode}`,{...bizOpts(apiKey),signal,data:{expected_revision:revision}})).data.data,
  reviewUpgrade: async(apiKey:string,currency:string,signal?:AbortSignal) => (await client.post<BizOne<BillingUpgradeReport>>('/biz/lifecycle/prepare',{primary_currency:currency,dry_run:true},{...bizOpts(apiKey),signal})).data.data,
  persistUpgradeReview: async(apiKey:string,report:BillingUpgradeReport,signal?:AbortSignal) => (await client.post<BizOne<BillingUpgradeReport>>('/biz/lifecycle/prepare',{primary_currency:report.primary_currency,dry_run:false,expected_report_id:report.id},{...bizOpts(apiKey),signal})).data.data,
  activateUpgrade: async(apiKey:string,reportID:string,signal?:AbortSignal) => (await client.post('/biz/lifecycle/activate',{report_id:reportID},{...bizOpts(apiKey),signal})).data.data,
  listProductsPage: async(apiKey:string,params:BillingPage={},signal?:AbortSignal) => (await client.get<BizListResponse<BizProduct>>('/biz/products',{...bizOpts(apiKey),params,signal})).data,
  listPlansPage: async(apiKey:string,params:BillingPage={},signal?:AbortSignal) => (await client.get<BizListResponse<BizPlan>>('/biz/plans',{...bizOpts(apiKey),params,signal})).data,
  // ── Products ──
  listProducts: async (apiKey: string) => {
    const { data } = await client.get<BizOne<BizProduct[]>>('/biz/products', bizOpts(apiKey));
    return data.data;
  },
  getProduct: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizProduct>>(`/biz/products/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createProduct: async (apiKey: string, payload: Partial<BizProduct>) => {
    const { data } = await client.post<BizOne<BizProduct>>('/biz/products', payload, bizOpts(apiKey));
    return data.data;
  },
  updateProduct: async (apiKey: string, id: string, payload: Partial<BizProduct>) => {
    const { data } = await client.put<BizOne<BizProduct>>(`/biz/products/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteProduct: async (apiKey: string, id: string) => {
    await client.delete(`/biz/products/${id}`, bizOpts(apiKey));
  },

  // ── Plans & Addons ──
  listPlans: async (apiKey: string) => {
    const { data } = await client.get<BizOne<BizPlan[]>>('/biz/plans', bizOpts(apiKey));
    return data.data;
  },
  getPlan: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizPlan>>(`/biz/plans/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createPlan: async (apiKey: string, payload: Partial<BizPlan>) => {
    const { data } = await client.post<BizOne<BizPlan>>('/biz/plans', payload, bizOpts(apiKey));
    return data.data;
  },
  updatePlan: async (apiKey: string, id: string, payload: Partial<BizPlan>) => {
    const { data } = await client.put<BizOne<BizPlan>>(`/biz/plans/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deletePlan: async (apiKey: string, id: string) => {
    await client.delete(`/biz/plans/${id}`, bizOpts(apiKey));
  },
  listPlanAddons: async (apiKey: string, planId: string) => {
    const { data } = await client.get<BizOne<BizPlanAddon[]>>(`/biz/plans/${planId}/addons`, bizOpts(apiKey));
    return data.data;
  },
  addPlanAddon: async (apiKey: string, planId: string, payload: Partial<BizPlanAddon>) => {
    const { data } = await client.post<BizOne<BizPlanAddon>>(`/biz/plans/${planId}/addons`, payload, bizOpts(apiKey));
    return data.data;
  },
  removePlanAddon: async (apiKey: string, planId: string, addonId: string) => {
    await client.delete(`/biz/plans/${planId}/addons/${addonId}`, bizOpts(apiKey));
  },

  // ── Config ──
  getConfig: async (apiKey: string, configType: string) => {
    const { data } = await client.get<BizOne<BizConfig>>(`/biz/config/${configType}`, bizOpts(apiKey));
    return data.data;
  },
  updateConfig: async (apiKey: string, configType: string, payload: Partial<BizConfig>) => {
    const { data } = await client.put<BizOne<BizConfig>>(`/biz/config/${configType}`, payload, bizOpts(apiKey));
    return data.data;
  },

  // ── Subscriptions ──
  listSubscriptions: async (apiKey: string, params?: { user_id?: string; plan_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizSubscription>>('/biz/subscriptions', { ...bizOpts(apiKey), params });
    return data;
  },
  getSubscription: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizSubscription>>(`/biz/subscriptions/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createSubscription: async (apiKey: string, payload: CreateBizSubscriptionRequest) => {
    const { data } = await client.post<BizOne<BizSubscription>>('/biz/subscriptions', payload, bizOpts(apiKey));
    return data.data;
  },
  updateSubscription: async (apiKey: string, id: string, payload: UpdateBizSubscriptionRequest) => {
    const { data } = await client.put<BizOne<BizSubscription>>(`/biz/subscriptions/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  changePlan: async (apiKey: string, id: string, payload: ChangeBizPlanRequest) => {
    const { data } = await client.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/change-plan`, payload, bizOpts(apiKey));
    return data.data;
  },
  cancelSubscription: async (apiKey: string, id: string, atPeriodEnd = false) => {
    const { data } = await client.post<BizOne<BizSubscription>>(
      `/biz/subscriptions/${id}/cancel`,
      {},
      { ...bizOpts(apiKey), params: { at_period_end: atPeriodEnd } },
    );
    return data.data;
  },
  pauseSubscription: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/pause`, {}, bizOpts(apiKey));
    return data.data;
  },
  resumeSubscription: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/resume`, {}, bizOpts(apiKey));
    return data.data;
  },
  reactivateSubscription: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizSubscription>>(`/biz/subscriptions/${id}/reactivate`, {}, bizOpts(apiKey));
    return data.data;
  },
  setNonRenewing: async (apiKey: string, id: string, nonRenewing: boolean) => {
    const { data } = await client.post<BizOne<BizSubscription>>(
      `/biz/subscriptions/${id}/non-renewing`,
      { non_renewing: nonRenewing },
      bizOpts(apiKey),
    );
    return data.data;
  },

  // ── Invoices ──
  listInvoices: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizInvoice>>('/biz/invoices', { ...bizOpts(apiKey), params });
    return data;
  },
  getInvoice: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizInvoice>>(`/biz/invoices/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createInvoice: async (apiKey: string, payload: CreateBizInvoiceRequest) => {
    const { data } = await client.post<BizOne<BizInvoice>>('/biz/invoices', payload, bizOpts(apiKey));
    return data.data;
  },
  updateDraftInvoice: async (apiKey: string, id: string, payload: UpdateBizInvoiceRequest) => {
    const { data } = await client.put<BizOne<BizInvoice>>(`/biz/invoices/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  issueInvoice: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/issue`, {}, bizOpts(apiKey));
    return data.data;
  },
  sendInvoice: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  voidInvoice: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/void`, {}, bizOpts(apiKey));
    return data.data;
  },
  writeOffInvoice: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/write-off`, {}, bizOpts(apiKey));
    return data.data;
  },
  applyCoupon: async (apiKey: string, id: string, payload: ApplyBizCouponRequest) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/apply-coupon`, payload, bizOpts(apiKey));
    return data.data;
  },
  applyCredit: async (apiKey: string, id: string, payload: ApplyBizCreditRequest = {}) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/apply-credit`, payload, bizOpts(apiKey));
    return data.data;
  },
  addLateFee: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizInvoice>>(`/biz/invoices/${id}/late-fee`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Payments ──
  recordPayment: async (apiKey: string, invoiceId: string, payload: RecordPaymentRequest) => {
    const { data } = await client.post<BizOne<BizPayment>>(`/biz/invoices/${invoiceId}/payments`, payload, bizOpts(apiKey));
    return data.data;
  },
  listPayments: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizPayment>>('/biz/payments', { ...bizOpts(apiKey), params });
    return data;
  },
  getPayment: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizPayment>>(`/biz/payments/${id}`, bizOpts(apiKey));
    return data.data;
  },
  refundPayment: async (apiKey: string, id: string, payload: RefundBizPaymentRequest = {}) => {
    const { data } = await client.post<BizOne<BizPayment>>(`/biz/payments/${id}/refund`, payload, bizOpts(apiKey));
    return data.data;
  },

  // ── Estimates ──
  listEstimates: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizEstimate>>('/biz/estimates', { ...bizOpts(apiKey), params });
    return data;
  },
  getEstimate: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizEstimate>>(`/biz/estimates/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createEstimate: async (apiKey: string, payload: CreateBizEstimateRequest) => {
    const { data } = await client.post<BizOne<BizEstimate>>('/biz/estimates', payload, bizOpts(apiKey));
    return data.data;
  },
  updateEstimate: async (apiKey: string, id: string, payload: Partial<CreateBizEstimateRequest>) => {
    const { data } = await client.put<BizOne<BizEstimate>>(`/biz/estimates/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteEstimate: async (apiKey: string, id: string) => {
    await client.delete(`/biz/estimates/${id}`, bizOpts(apiKey));
  },
  sendEstimate: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  acceptEstimate: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/accept`, {}, bizOpts(apiKey));
    return data.data;
  },
  rejectEstimate: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizEstimate>>(`/biz/estimates/${id}/reject`, {}, bizOpts(apiKey));
    return data.data;
  },
  convertEstimate: async (apiKey: string, id: string, lineIndexes?: number[]) => {
    const { data } = await client.post<BizOne<BizInvoice>>(
      `/biz/estimates/${id}/convert`,
      lineIndexes?.length ? { line_indexes: lineIndexes } : {},
      bizOpts(apiKey),
    );
    return data.data;
  },

  // ── Coupons ──
  listCoupons: async (apiKey: string, params?: { code?: string; active?: boolean }) => {
    const { data } = await client.get<BizListResponse<BizCoupon>>('/biz/coupons', { ...bizOpts(apiKey), params });
    return data;
  },
  getCoupon: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizCoupon>>(`/biz/coupons/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createCoupon: async (apiKey: string, payload: CreateBizCouponRequest) => {
    const { data } = await client.post<BizOne<BizCoupon>>('/biz/coupons', payload, bizOpts(apiKey));
    return data.data;
  },
  bulkGenerateCoupons: async (apiKey: string, payload: BulkGenerateBizCouponsRequest) => {
    const { data } = await client.post<BizOne<BizCoupon[]>>('/biz/coupons/bulk-generate', payload, bizOpts(apiKey));
    return data.data;
  },
  updateCoupon: async (apiKey: string, id: string, payload: Partial<CreateBizCouponRequest> & { active?: boolean }) => {
    const { data } = await client.put<BizOne<BizCoupon>>(`/biz/coupons/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteCoupon: async (apiKey: string, id: string) => {
    await client.delete(`/biz/coupons/${id}`, bizOpts(apiKey));
  },

  // ── Credit notes & retainers ──
  listCreditNotes: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizCreditNote>>('/biz/credit-notes', { ...bizOpts(apiKey), params });
    return data;
  },
  getCreditNote: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizCreditNote>>(`/biz/credit-notes/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createCreditNote: async (apiKey: string, payload: CreateBizCreditNoteRequest) => {
    const { data } = await client.post<BizOne<BizCreditNote>>('/biz/credit-notes', payload, bizOpts(apiKey));
    return data.data;
  },
  refundCreditNote: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizCreditNote>>(`/biz/credit-notes/${id}/refund`, {}, bizOpts(apiKey));
    return data.data;
  },
  listRetainers: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizRetainer>>('/biz/retainers', { ...bizOpts(apiKey), params });
    return data;
  },
  getRetainer: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizRetainer>>(`/biz/retainers/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createRetainer: async (apiKey: string, payload: CreateBizRetainerRequest) => {
    const { data } = await client.post<BizOne<BizRetainer>>('/biz/retainers', payload, bizOpts(apiKey));
    return data.data;
  },
  markRetainerPaid: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizRetainer>>(`/biz/retainers/${id}/mark-paid`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Contracts ──
  listContracts: async (apiKey: string, params?: { user_id?: string; status?: string }) => {
    const { data } = await client.get<BizListResponse<BizContract>>('/biz/contracts', { ...bizOpts(apiKey), params });
    return data;
  },
  getContract: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizContract>>(`/biz/contracts/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createContract: async (apiKey: string, payload: CreateBizContractRequest) => {
    const { data } = await client.post<BizOne<BizContract>>('/biz/contracts', payload, bizOpts(apiKey));
    return data.data;
  },
  updateContract: async (apiKey: string, id: string, payload: Partial<CreateBizContractRequest>) => {
    const { data } = await client.put<BizOne<BizContract>>(`/biz/contracts/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  sendContract: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizContract>>(`/biz/contracts/${id}/send`, {}, bizOpts(apiKey));
    return data.data;
  },
  acceptContract: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizContract>>(`/biz/contracts/${id}/accept`, {}, bizOpts(apiKey));
    return data.data;
  },
  amendContract: async (apiKey: string, id: string, payload: AmendBizContractRequest) => {
    const { data } = await client.post<BizOne<BizContract>>(`/biz/contracts/${id}/amend`, payload, bizOpts(apiKey));
    return data.data;
  },
  renewContract: async (apiKey: string, id: string, newEndDate?: string) => {
    const { data } = await client.post<BizOne<BizContract>>(
      `/biz/contracts/${id}/renew`,
      newEndDate ? { new_end_date: newEndDate } : {},
      bizOpts(apiKey),
    );
    return data.data;
  },
  terminateContract: async (apiKey: string, id: string) => {
    const { data } = await client.post<BizOne<BizContract>>(`/biz/contracts/${id}/terminate`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Usage ──
  listUsageMeters: async (apiKey: string) => {
    const { data } = await client.get<BizOne<BizUsageMeter[]>>('/biz/usage/meters', bizOpts(apiKey));
    return data.data;
  },
  createUsageMeter: async (apiKey: string, payload: CreateBizUsageMeterRequest) => {
    const { data } = await client.post<BizOne<BizUsageMeter>>('/biz/usage/meters', payload, bizOpts(apiKey));
    return data.data;
  },
  reportUsage: async (apiKey: string, payload: ReportBizUsageRequest) => {
    const { data } = await client.post<{ success: boolean; accepted: number }>('/biz/usage/events', payload, bizOpts(apiKey));
    return data.accepted;
  },
  getUsageSummary: async (
    apiKey: string,
    params: { user_id: string; subscription_id?: string; from?: string; to?: string },
  ) => {
    const { data } = await client.get<BizOne<BizUsageSummary[]>>('/biz/usage/summary', { ...bizOpts(apiKey), params });
    return data.data;
  },

  // ── Expenses ──
  listExpenses: async (apiKey: string, params?: { category?: string; user_id?: string; billable?: boolean }) => {
    const { data } = await client.get<BizListResponse<BizExpense>>('/biz/expenses', { ...bizOpts(apiKey), params });
    return data;
  },
  getExpense: async (apiKey: string, id: string) => {
    const { data } = await client.get<BizOne<BizExpense>>(`/biz/expenses/${id}`, bizOpts(apiKey));
    return data.data;
  },
  createExpense: async (apiKey: string, payload: CreateBizExpenseRequest) => {
    const { data } = await client.post<BizOne<BizExpense>>('/biz/expenses', payload, bizOpts(apiKey));
    return data.data;
  },
  updateExpense: async (apiKey: string, id: string, payload: Partial<CreateBizExpenseRequest>) => {
    const { data } = await client.put<BizOne<BizExpense>>(`/biz/expenses/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteExpense: async (apiKey: string, id: string) => {
    await client.delete(`/biz/expenses/${id}`, bizOpts(apiKey));
  },
  convertExpensesToInvoice: async (apiKey: string, payload: { user_id: string; expense_ids: string[] }) => {
    const { data } = await client.post<BizOne<BizInvoice>>('/biz/expenses/convert', payload, bizOpts(apiKey));
    return data.data;
  },
  getExpenseSummary: async (apiKey: string, params?: { from?: string; to?: string }) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/expenses/summary', { ...bizOpts(apiKey), params });
    return data.data;
  },

  // ── Customer tools ──
  getUserStatement: async (apiKey: string, userId: string) => {
    const { data } = await client.get<BizOne<BizStatement>>(`/biz/users/${userId}/statement`, bizOpts(apiKey));
    return data.data;
  },
  adjustUserBalance: async (apiKey: string, userId: string, payload: { amount_paisa: number; reason?: string }) => {
    const { data } = await client.post<BizOne<{ balance_paisa: number }>>(
      `/biz/users/${userId}/balance`,
      payload,
      bizOpts(apiKey),
    );
    return data.data;
  },
  createPortalLink: async (apiKey: string, userId: string) => {
    const { data } = await client.post<BizOne<BizPortalLink>>(`/biz/users/${userId}/portal-link`, {}, bizOpts(apiKey));
    return data.data;
  },

  // ── Analytics ──
  analyticsRevenue: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/revenue', bizOpts(apiKey));
    return data.data;
  },
  analyticsSubscriptions: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/subscriptions', bizOpts(apiKey));
    return data.data;
  },
  analyticsInvoices: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/invoices', bizOpts(apiKey));
    return data.data;
  },
  analyticsAging: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/aging', bizOpts(apiKey));
    return data.data;
  },
  analyticsCustomers: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>[]>>('/biz/analytics/customers', bizOpts(apiKey));
    return data.data;
  },
  analyticsDunning: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/dunning', bizOpts(apiKey));
    return data.data;
  },
  analyticsRevenueRecognition: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/revenue-recognition', bizOpts(apiKey));
    return data.data;
  },
  analyticsTaxSummary: async (apiKey: string) => {
    const { data } = await client.get<BizOne<Record<string, unknown>>>('/biz/analytics/tax-summary', bizOpts(apiKey));
    return data.data;
  },

  // ── Connectors ──
  listConnectors: async (apiKey: string) => {
    const { data } = await client.get<BizOne<BizConnector[]>>('/biz/connectors', bizOpts(apiKey));
    return data.data;
  },
  createConnector: async (apiKey: string, payload: CreateBizConnectorRequest) => {
    const { data } = await client.post<BizOne<BizConnector>>('/biz/connectors', payload, bizOpts(apiKey));
    return data.data;
  },
  updateConnector: async (apiKey: string, id: string, payload: UpdateBizConnectorRequest) => {
    const { data } = await client.put<BizOne<BizConnector>>(`/biz/connectors/${id}`, payload, bizOpts(apiKey));
    return data.data;
  },
  deleteConnector: async (apiKey: string, id: string) => {
    await client.delete(`/biz/connectors/${id}`, bizOpts(apiKey));
  },
};

}
