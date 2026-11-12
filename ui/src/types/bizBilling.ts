// ============= Business Billing Types =============
export interface BizProduct {
    id?: string;
    app_id?: string;
    name: string;
    description: string;
    active: boolean;
    hsn_code?: string;
    tax_rate?: number;
    unit?: string;
    metadata?: Record<string, any>;
    created_at?: string;
    updated_at?: string;
}

export interface BizPlan {
    id?: string;
    app_id?: string;
    product_id: string;
    name: string;
    description: string;
    billing_cycle: 'monthly' | 'yearly' | 'one-time' | string;
    amount_paisa: number;
    currency: string;
    trial_days?: number;
    active: boolean;
    pricing_model?: 'flat' | 'per_unit' | 'tiered' | 'volume';
    cycle_days?: number;
    setup_fee_paisa?: number;
    tax_inclusive?: boolean;
    tiers?: { up_to: number; unit_paisa: number; flat_paisa?: number }[];
    limits?: Record<string, any>;
    features?: string[];
    created_at?: string;
    updated_at?: string;
}

export interface BizPlanAddon {
    id?: string;
    plan_id?: string;
    name: string;
    description: string;
    amount_paisa: number;
    currency?: string;
    billing_cycle?: string;
    created_at?: string;
    updated_at?: string;
}

export interface BizConfig {
    app_id?: string;
    config_type: 'stripe' | 'razorpay' | 'paddle' | string;
    settings?: Record<string, any>;
    data?: Record<string, any>;
    updated_at?: string;
}

export interface BizSubscription {
    id: string;
    app_id: string;
    user_id: string;
    plan_id: string;
    addon_ids?: string[];
    status: 'active' | 'paused' | 'canceled' | 'trialing' | 'past_due' | 'unpaid' | 'awaiting_authorization';
    currency?: string;
    gateway_mode?: BillingMode;
    collection_mode?: 'invoice' | 'automatic';
    collection_state?: string;
    automatic_cycles?: number;
    pending_change?: { plan_id?: string; quantity?: number; effective_at?: string };
    quantity: number;
    cancel_at_period_end?: boolean;
    current_period_start: string;
    current_period_end: string;
    trial_start?: string;
    trial_end?: string;
    paused_at?: string;
    canceled_at?: string;
    plan_name?: string;
    plan_amount_paisa?: number;
    billing_cycle?: string;
    created_at: string;
}

export interface BizInvoice {
    id: string;
    app_id: string;
    user_id: string;
    subscription_id?: string;
    invoice_number: string;
    status: 'draft' | 'open' | 'partially_paid' | 'paid' | 'void' | 'uncollectible' | 'overdue';
    amount_paid_paisa?: number;
    credits_applied?: number;
    cash_refunded_paisa?: number;
    subtotal_paisa?: number;
    discount_paisa?: number;
    tax_amount_paisa?: number;
    late_fee_paisa?: number;
    collection_owner?: string;
    gateway_mode?: BillingMode;
    line_items?: BizInvoiceLineItem[];
    due_date?: string;
    issued_snapshot?: { customer?: BillingIdentity; issuer?: BillingIdentity };
    total_paisa: number;
    amount_due_paisa: number;
    currency: string;
    payment_link?: string;
    issued_at?: string;
    created_at: string;
}

export interface BizPayment {
    refunds?:{id:string;reference?:string;amount_paisa:number;state:string}[];
    user_id?: string;
    app_id?: string;
    reference?: string;
    refunded_paisa?: number;
    refund_reserved_paisa?: number;
    excess_credit_paisa?: number;
    credit_provenance_known?: boolean;
    id: string;
    invoice_ids: string[];
    amount_paisa: number;
    currency: string;
    method: string;
    status: 'pending' | 'success' | 'failed' | 'refunded' | 'partially_refunded';
    gateway_payment_id?: string;
    created_at: string;
}

// ── Phase 2: Request Types ──

export interface CreateBizSubscriptionRequest {
    collection_mode?: 'invoice' | 'automatic';
    automatic_cycles?: number;
    gateway_mode?: BillingMode;
    user_id: string;
    plan_id: string;
    quantity?: number;
    addon_ids?: string[];
    trial_days?: number;
    metadata?: Record<string, any>;
}

export interface UpdateBizSubscriptionRequest {
    change_timing?: 'immediate' | 'next_period';
    plan_id?: string;
    quantity?: number;
    addon_ids?: string[];
    cancel_at_period_end?: boolean;
    metadata?: Record<string, any>;
}

export interface CreateBizInvoiceRequest {
    notes?: string;
    gateway_mode?: BillingMode;
    user_id: string;
    subscription_id?: string;
    line_items?: BizInvoiceLineItem[];
    currency?: string;
    payment_terms?: string;
    due_days?: number;
    metadata?: Record<string, any>;
}

export interface UpdateBizInvoiceRequest {
    line_items?: BizInvoiceLineItem[];
    metadata?: Record<string, any>;
}

export interface BizInvoiceLineItem {
    description: string;
    hsn_code?: string;
    quantity: number;
    /** Unit price in paisa (matches backend `unit_price`). */
    unit_price: number;
    discount_paisa?: number;
    tax_rate?: number;
    tax_rate_override?: number;
    tax_exempt?: boolean;
    amount_paisa?: number;
}

export interface RecordPaymentRequest {
    reference?: string;
    method: string;
    gateway_payment_id?: string;
    amount_paisa?: number;
    gateway_txn_id?: string;
    tds_paisa?: number;
    notes?: string;
}

export interface ChangeBizPlanRequest {
    change_timing?: 'immediate' | 'next_period';
    plan_id: string;
    quantity?: number;
}

export interface RefundBizPaymentRequest {
    amount_paisa?: number;
    reason?: string;
}

export interface ApplyBizCouponRequest {
    code: string;
}

export interface ApplyBizCreditRequest {
    credit_note_id?: string;
    retainer_id?: string;
}

export interface BizEstimate {
    id: string;
    app_id: string;
    user_id: string;
    estimate_number: string;
    status: 'draft' | 'sent' | 'accepted' | 'rejected' | 'expired' | 'converted' | string;
    line_items?: BizInvoiceLineItem[];
    subtotal_paisa: number;
    tax_paisa: number;
    total_paisa: number;
    currency: string;
    valid_until?: string;
    notes?: string;
    terms?: string;
    converted_invoice_id?: string;
    created_at: string;
    updated_at?: string;
}

export interface CreateBizEstimateRequest {
    user_id: string;
    line_items: BizInvoiceLineItem[];
    currency?: string;
    valid_until?: string;
    notes?: string;
    terms?: string;
    metadata?: Record<string, any>;
}

export interface BizCoupon {
    id: string;
    app_id: string;
    code: string;
    discount_type: 'percentage' | 'fixed' | string;
    discount_value: number;
    max_uses?: number;
    current_uses: number;
    max_uses_per_user?: number;
    applicable_plan_ids?: string[];
    stackable: boolean;
    valid_from?: string;
    valid_until?: string;
    active: boolean;
    created_at: string;
}

export interface CreateBizCouponRequest {
    code: string;
    discount_type: 'percentage' | 'fixed' | string;
    discount_value: number;
    max_uses?: number;
    max_uses_per_user?: number;
    applicable_plan_ids?: string[];
    stackable?: boolean;
    valid_from?: string;
    valid_until?: string;
}

export interface BulkGenerateBizCouponsRequest {
    count: number;
    prefix?: string;
    rules: CreateBizCouponRequest;
}

export interface BizCreditNote {
    id: string;
    app_id: string;
    user_id: string;
    invoice_id?: string;
    credit_note_number: string;
    amount_paisa: number;
    balance_paisa: number;
    reason?: string;
    status: 'open' | 'applied' | 'refunded' | 'void' | string;
    created_at: string;
}

export interface CreateBizCreditNoteRequest {
    user_id: string;
    invoice_id?: string;
    amount_paisa: number;
    reason?: string;
}

export interface BizRetainer {
    id: string;
    app_id: string;
    user_id: string;
    amount_paisa: number;
    balance_paisa: number;
    status: 'open' | 'paid' | 'applied' | 'void' | string;
    notes?: string;
    created_at: string;
}

export interface CreateBizRetainerRequest {
    user_id: string;
    amount_paisa: number;
    notes?: string;
}

export interface BizContract {
    id: string;
    app_id: string;
    user_id: string;
    subscription_id?: string;
    contract_number: string;
    status: 'draft' | 'sent' | 'active' | 'expired' | 'terminated' | string;
    terms?: string;
    value_paisa: number;
    start_date: string;
    end_date: string;
    auto_renew: boolean;
    created_at: string;
}

export interface CreateBizContractRequest {
    user_id: string;
    subscription_id?: string;
    terms?: string;
    value_paisa?: number;
    start_date: string;
    end_date: string;
    auto_renew?: boolean;
    metadata?: Record<string, any>;
}

export interface AmendBizContractRequest {
    description: string;
    effective_date: string;
}

export interface BizUsageMeter {
    id: string;
    app_id: string;
    name: string;
    unit?: string;
    aggregation: string;
    created_at: string;
}

export interface CreateBizUsageMeterRequest {
    name: string;
    unit?: string;
    aggregation?: string;
}

export interface ReportBizUsageRequest {
    events: Array<{
        user_id: string;
        subscription_id?: string;
        meter_id: string;
        quantity: number;
        timestamp?: string;
    }>;
}

export interface BizUsageSummary {
    meter_id: string;
    meter_name?: string;
    unit?: string;
    aggregation: string;
    quantity: number;
    period_start: string;
    period_end: string;
}

export interface BizExpense {
    id: string;
    app_id: string;
    category: string;
    description?: string;
    amount_paisa: number;
    vendor?: string;
    date: string;
    billable: boolean;
    billed_to_user?: string;
    invoice_id?: string;
    recurring?: boolean;
    recurrence_cycle?: string;
    created_at: string;
}

export interface CreateBizExpenseRequest {
    category: string;
    description?: string;
    amount_paisa: number;
    vendor?: string;
    date?: string;
    billable?: boolean;
    billed_to_user?: string;
    receipt_file_id?: string;
    recurring?: boolean;
    recurrence_cycle?: string;
    metadata?: Record<string, any>;
}

export interface BizConnector {
    id: string;
    app_id: string;
    provider: string;
    webhook_secret?: string;
    event_mappings?: Record<string, any>;
    active: boolean;
    created_at: string;
}

export interface CreateBizConnectorRequest {
    provider: string;
    webhook_secret: string;
    event_mappings?: Record<string, any>;
}

export interface UpdateBizConnectorRequest {
    webhook_secret?: string;
    event_mappings?: Record<string, any>;
    active?: boolean;
}

export interface BizStatement {
    currency?: string;
    user_id: string;
    entries: Array<{
        date: string;
        type: string;
        reference: string;
        description?: string;
        debit_paisa?: number;
        credit_paisa?: number;
    }>;
    total_billed_paisa: number;
    total_paid_paisa: number;
    balance_owed_paisa: number;
    credit_balance_paisa: number;
    generated_at: string;
}

export interface BizPortalLink {
    url: string;
    expires_at: string;
}

export interface BizListResponse<T> {
    success: boolean;
    data: T[];
    total?: number;
}

export type BillingMode = 'test' | 'live';
export interface BillingLifecycle {
    app_id: string;
    lifecycle_version: number;
    status: string;
    primary_currency?: string;
    new_collection_enabled: boolean;
    active_writers: number;
    migration_report_id?: string;
}
export interface BillingMerchant {
    app_id: string;
    configured: boolean;
    mode: BillingMode;
    revision: number;
    account_id?: string;
    connection_id?: string;
    readiness: string;
    api_authenticated: boolean;
    account_verified: boolean;
    automatic_ready: boolean;
}
export interface BillingMerchantInput {
    expected_revision: number;
    account_id?: string;
    key_id?: string;
    key_secret?: string;
    webhook_secret?: string;
}
export interface BillingUpgradeReport {
    id: string;
    app_id: string;
    primary_currency: string;
    ready: boolean;
    writers_upgraded: boolean;
    checkpoint: string;
    resource_counts: Record<string, number>;
    issues: { id: string; index: string; resource_id: string; code: string; source_hash: string }[];
    legacy_gateway_bindings: { kind: string; id: string }[];
    opening_balances: { scope: { user_id: string; currency: string }; available_paisa: number }[];
}
export interface BillingPreferences { default_currency: string; supported_currencies: string[] }
export interface BillingBranding {
    company_name?: string; email?: string; phone?: string; primary_color?: string;
    address?: { line1?: string; line2?: string; city?: string; state?: string; pincode?: string; country?: string };
    logo_file_id?: string; footer_text?: string; terms_text?: string;
}
export interface BillingTax { default_tax_rate: number; tax_inclusive: boolean; business_gstin?: string; business_state?: string; e_invoice_enabled?: boolean }
export interface BillingNumbering {
    invoice_prefix?: string; estimate_prefix?: string; credit_note_prefix?: string; contract_prefix?: string;
    next_invoice_number?: number; next_estimate_number?: number; next_credit_note_number?: number; next_contract_number?: number;
}
export interface BillingPaymentTerms { default_terms: string; custom_days?: number }
export interface BillingDunning { enabled: boolean; steps?: { day_offset: number; channel: string; template: string }[]; auto_action?: string; auto_action_day?: number }
export interface BillingConfigMap {
    preferences: BillingPreferences; branding: BillingBranding; tax: BillingTax;
    numbering: BillingNumbering; payment_terms: BillingPaymentTerms; dunning: BillingDunning;
}
export interface BillingPage { limit?: number; offset?: number; search?: string; active?: boolean; product_id?: string }
export interface BillingIdentity { name?:string; full_name?:string; email?:string; phone?:string; address?:Record<string,string>; gstin?:string }
export interface BillingDocumentData extends BizInvoice { invoice_id:string; issuer:BillingIdentity; customer:BillingIdentity; payments:BizPayment[]; credits:unknown[]; notes?:string; footer_text?:string; terms_text?:string }
export interface BillingInvoicePreview {currency:string;subtotal_paisa:number;discount_paisa:number;tax_amount_paisa:number;total_paisa:number;line_items:BizInvoiceLineItem[]}
export interface BillingCreditBalance {available_paisa:number;reserved_paisa:number;unresolved_legacy:boolean}
export interface BillingDelivery {id:string;status:string;failure_code?:string;attempts:number;may_have_dispatched:boolean;intent:{user_id:string;template_id:string;invoice_id?:string;channel:string}}
