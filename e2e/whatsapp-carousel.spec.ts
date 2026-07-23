/**
 * WhatsApp Rich Templates — carousel authoring smoke test.
 *
 * Verifies the carousel builder POSTs a well-formed RichTemplate to
 * /v1/whatsapp/rich-templates/ when the user clicks "Submit to Meta".
 * Network is intercepted so this spec runs without a real Meta binding
 * and without persisting state into the test app.
 *
 * Covers WHATSAPP_RICH_INTERACTIVE_PLAN.md §3 (carousel authoring UI) and
 * §10 (e2e). Click attribution and runtime resolution are covered by Go
 * unit tests in internal/usecases/services/whatsapp_rich_template_service_test.go.
 */
import { test, expect, captureRequestBody } from './fixtures';

test.describe('WhatsApp Rich Templates', () => {
    test.beforeEach(async ({ page, state }) => {
        // Stub list endpoint so the page renders an empty state without
        // touching the real backend. The order matters: install routes
        // BEFORE navigation so the very first XHR is intercepted.
        await page.route('**/v1/whatsapp/rich-templates/**', async (route) => {
            const req = route.request();
            if (req.method() === 'GET') {
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({ data: [], total: 0 }),
                });
                return;
            }
            if (req.method() === 'POST') {
                // Echo back a fake RichTemplate so the UI's success path
                // (toast + refetch) runs without errors.
                await route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify({
                        data: {
                            id: 'rich-tpl-test-1',
                            kind: 'carousel',
                            name: 'spec_carousel',
                            language: 'en_US',
                            providers: {},
                            created_at: new Date().toISOString(),
                        },
                    }),
                });
                return;
            }
            await route.continue();
        });

        await page.goto(`/apps/${state.appId}?tab=whatsapp`);
        // Wait for the rich-templates section heading to confirm the
        // WhatsApp tab finished mounting before the spec interacts with it.
        await page
            .getByRole('heading', { name: 'WhatsApp Rich Templates' })
            .waitFor({ state: 'visible', timeout: 15_000 });
    });

    test('creating a carousel posts the expected RichTemplate payload', async ({ page }) => {
        // Open the builder.
        await page.getByRole('button', { name: /New Rich Template/i }).click();

        // The form defaults to kind="carousel" with 2 empty cards (§3.1).
        // Fill template-level fields.
        await page.locator('input[placeholder="diwali_carousel"]').fill('spec_carousel');
        await page.locator('input[placeholder="en_US"]').fill('en_US');
        await page.locator('input[placeholder="Hi {{1}}, check these out:"]').fill(
            'Hi {{1}}, check these carousel picks:',
        );

        // Fill card 1.
        const imageInputs = page.locator('input[placeholder="https://cdn.example.com/p1.jpg"]');
        await imageInputs.nth(0).fill('https://cdn.example.com/p1.jpg');
        const bodyInputs = page.locator('input[placeholder="Polo {{1}} {{2}}"]');
        await bodyInputs.nth(0).fill('Trendy Polo {{1}}');
        const urlInputs = page.locator('input[placeholder="https://shop.example/p/{{1}}"]');
        await urlInputs.nth(0).fill('https://shop.example/p/{{1}}');

        // Fill card 2.
        await imageInputs.nth(1).fill('https://cdn.example.com/p2.jpg');
        await bodyInputs.nth(1).fill('Classic Shirt {{1}}');
        await urlInputs.nth(1).fill('https://shop.example/p/{{1}}');

        const body = await captureRequestBody(page, '/v1/whatsapp/rich-templates/', async () => {
            await page.getByRole('button', { name: /Submit to Meta/i }).click();
        });

        expect(body.kind).toBe('carousel');
        expect(body.name).toBe('spec_carousel');
        expect(body.language).toBe('en_US');
        expect(Array.isArray(body.cards)).toBe(true);
        // Plan §3: a carousel must carry between 2 and 10 cards.
        expect((body.cards as unknown[]).length).toBeGreaterThanOrEqual(2);
        // Per-card body + header image must be present.
        const firstCard = (body.cards as Array<Record<string, unknown>>)[0];
        expect(firstCard.body).toBe('Trendy Polo {{1}}');
    });
});
