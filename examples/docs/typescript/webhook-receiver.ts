/** Verify the generic outbound webhook's exact raw bytes before JSON parsing.
 * A missing secret fails closed unless an operator explicitly allows unsigned delivery.
 * Custom-provider v2 and inbound/vendor webhooks have different signing contracts.
 */
export async function verifyWebhook(
 rawBody: Uint8Array, signature: string | null, secret: string | undefined,
 allowUnsigned = false,
): Promise<boolean> {
 if (!secret) return allowUnsigned && !signature;
 if (!signature || !/^[a-f0-9]{64}$/i.test(signature)) return false;
 const bytes = Uint8Array.from(signature.match(/../g)!, pair => Number.parseInt(pair,16));
 const key = await crypto.subtle.importKey('raw', new TextEncoder().encode(secret),
  {name:'HMAC',hash:'SHA-256'}, false, ['verify']);
 return crypto.subtle.verify('HMAC', key, bytes.buffer, new Uint8Array(rawBody).buffer);
}
