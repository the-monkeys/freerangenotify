import { test, expect } from './fixtures';

for (const width of [360, 390, 768, 1280]) {
  for (const [slug, heading] of [
    ['business-billing', 'Billing your customers'],
    ['business-billing-payments', 'Receiving and returning payments'],
    ['customer-billing-portal', 'Your customer billing portal'],
    ['observability', 'Monitoring and privacy'],
  ]) {
    test(`${slug} is readable at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 });
      await page.goto(`/docs/${slug}`);
      await expect(page.getByRole('heading', { name: heading, exact: true })).toBeVisible();
      const overflow = await page.evaluate(() => [...document.querySelectorAll('article p, article li, article code')]
        .filter(el => el.getBoundingClientRect().right > window.innerWidth)
        .map(el => ({ tag: el.tagName, text: el.textContent?.slice(0, 160), right: el.getBoundingClientRect().right })));
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), JSON.stringify(overflow)).toBe(true);
      if (slug === 'business-billing-payments' && [390, 1280].includes(width)) {
        await page.screenshot({ path: testInfo.outputPath('payments-guide.png'), fullPage: true });
      }
    });
  }
}
