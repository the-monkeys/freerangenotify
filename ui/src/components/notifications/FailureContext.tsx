import type { NotificationMetadata } from '../../types';
import { isCreditAmount } from '../../lib/billingAvailability';

const networkExplanation = 'Network connection failed. Check the provider host and worker DNS connectivity.';
const explanations = new Map([
    ['network', networkExplanation],
    ['network_error', networkExplanation],
    ['timeout', 'The provider request timed out. Check connectivity and provider availability.'],
    ['authentication', 'Authentication failed. Check the credentials used by this attempt.'],
    ['configuration', 'Provider configuration was invalid or incomplete. Check the configured provider settings.'],
    ['invalid_request', 'Provider rejected the request as invalid. Check its content and recipient.'],
    ['rate_limit', 'Provider rate limit reached for this attempt.'],
    ['provider_api', 'Provider API reported a delivery failure. See the recorded error for details.'],
    ['unknown', 'Unclassified provider failure. See the recorded error for details.'],
    ['credits_temporarily_reserved', 'Credits were temporarily reserved by other attempts.'],
    ['insufficient_credits', 'Insufficient remaining credits for this attempt.'],
    ['daily_cap_exceeded', 'Daily channel limit reached for this attempt.'],
    ['notification_expired', 'The notification validity window expired before delivery.'],
]);
const credentialLabels = new Map([
    ['byoc', 'Application credentials'], ['system', 'System credentials'], ['platform', 'Platform credentials'],
]);
const creditFields = [
    ['credits_required', 'Required'], ['credits_remaining', 'Remaining'],
    ['credits_reserved', 'Reserved'], ['credits_available', 'Available'],
] as const;
const diagnosticFields = ['failure_code', 'failure_stage', 'provider', 'credential_source', 'rate_card_version'] as const;

export default function FailureContext({ metadata }: { metadata?: NotificationMetadata }) {
    if (!metadata) return null;
    const text = (key: typeof diagnosticFields[number]) => typeof metadata[key] === 'string' && metadata[key].trim() ? metadata[key] : undefined;
    const credits = creditFields.filter(([key]) => isCreditAmount(metadata[key]));
    if (!diagnosticFields.some(key => text(key)) && typeof metadata.retryable !== 'boolean' && !credits.length) return null;
    const explanation = explanations.get(text('failure_code') ?? '');

    return (
        <section aria-label="Failure context" className="rounded-lg border border-border bg-muted/30 p-3 space-y-2 text-sm">
            <p className="font-medium">Failure context</p>
            {text('failure_code') && <p>Code: <span className="font-mono break-words">{text('failure_code')}</span></p>}
            {explanation && <p>{explanation}</p>}
            {text('failure_stage') && <p>Stage: {text('failure_stage')}</p>}
            {text('provider') && <p>Provider: {text('provider')}</p>}
            {text('credential_source') && <p>Credential source: {credentialLabels.get(text('credential_source')!) ?? text('credential_source')}</p>}
            {typeof metadata.retryable === 'boolean' && <p>Retryable: {metadata.retryable ? 'Yes' : 'No'}</p>}
            {(credits.length > 0 || text('rate_card_version')) && (
                <div className="border-t border-border pt-2 space-y-1">
                    <p className="font-medium">Attempt-time credit snapshot</p>
                    <p className="text-xs text-muted-foreground">Recorded for this attempt. The current wallet may have changed since then.</p>
                    {credits.map(([key, label]) => <p key={key}>{label}: {metadata[key]!.toLocaleString()} credits</p>)}
                    {text('rate_card_version') && <p>Rate card: {text('rate_card_version')}</p>}
                </div>
            )}
        </section>
    );
}
