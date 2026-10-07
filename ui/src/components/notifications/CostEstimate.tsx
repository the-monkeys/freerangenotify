import type { BillingRates, BillingUsage } from '../../types';
import { availableCredits, isCreditAmount } from '../../lib/billingAvailability';

interface CostEstimateProps {
    rates: BillingRates | null;
    usage: BillingUsage | null;
    billingEnabled?: boolean;
    channel?: string;
    recipientCount?: number;
    workflow?: boolean;
    loading: boolean;
}

export default function CostEstimate({ rates, usage, billingEnabled, channel, recipientCount, workflow, loading }: CostEstimateProps) {
    const rateChannel = channel === 'in_app' ? 'inapp' : channel;
    const cost = rateChannel ? rates?.channel_credit_cost?.[rateChannel] : undefined;
    const available = availableCredits(usage, billingEnabled);
    const total = isCreditAmount(cost) && recipientCount !== undefined ? cost * recipientCount : undefined;
    let unavailable: string | undefined;
    if (billingEnabled === false) unavailable = 'Billing is disabled. Credit estimates do not apply.';
    else if (usage?.billing_model === 'legacy') unavailable = 'Legacy billing uses message quotas. Credit estimates do not apply.';
    else if (workflow) unavailable = 'Workflow cost depends on its steps; a single-channel estimate is unavailable.';
    else if (!channel) unavailable = 'Select a template to see its channel cost.';
    else if (loading) unavailable = 'Loading cost estimate…';
    else if (!isCreditAmount(cost)) unavailable = 'Cost estimate unavailable. Sending is still available.';

    return (
        <div role="status" aria-label="Cost estimate" className="rounded-lg border border-border bg-background p-3 text-sm space-y-1">
            {unavailable ? <p className="text-muted-foreground">{unavailable}</p> : (
                <>
                    <p>{cost!.toLocaleString()} credits per recipient ({channel}).</p>
                    {total !== undefined && Number.isFinite(total) ? (
                        <p className="font-medium">Estimated total: {total.toLocaleString()} credits for {recipientCount} recipient{recipientCount === 1 ? '' : 's'}.</p>
                    ) : <p className="text-muted-foreground">Recipient count is unknown; total cost is unavailable.</p>}
                    {available !== null ? <p>Currently available: {available.toLocaleString()} credits.</p> : <p className="text-muted-foreground">Current availability is unavailable.</p>}
                    {total !== undefined && available !== null && total > available && (
                        <p className="text-amber-700 dark:text-amber-400">This estimate exceeds currently available credits. Availability may change before processing.</p>
                    )}
                    {rates?.active_version && <p className="text-xs text-muted-foreground">Rate card: {rates.active_version}</p>}
                </>
            )}
            <p className="text-xs text-muted-foreground">Advisory only. Actual charges depend on credentials, billing mode, and processing-time availability. Sending does not guarantee delivery.</p>
        </div>
    );
}
