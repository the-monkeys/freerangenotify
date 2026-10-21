import {test,expect} from './fixtures';
import {readFileSync,readdirSync} from 'node:fs';
const nav=readFileSync('ui/src/config/docsNav.tsx','utf8');
const slugs=[...nav.matchAll(/to: '\/docs\/([^']+)'/g)].map(m=>m[1]).filter(s=>s!=='api');
test('every public guide and internal cross-link resolves logged out',async({page})=>{test.setTimeout(120000);const targets=new Set(slugs);for(const f of readdirSync('ui/src/docs')){if(f.endsWith('.md'))for(const m of readFileSync(`ui/src/docs/${f}`,'utf8').matchAll(/\]\(\/docs\/([^\s)#]+)(?:#[^)]*)?\)/g))if(m[1]!=='api')targets.add(m[1]);}for(const slug of targets){await page.goto(`/docs/${slug}`);await expect(page.locator('article h1')).toBeVisible();await expect(page.getByText('Document not found',{exact:true})).toHaveCount(0);}});
for(const theme of ['light','dark'])test(`mobile guide ${theme}`,async({page})=>{await page.setViewportSize({width:390,height:844});await page.addInitScript(t=>localStorage.setItem('theme',t),theme);await page.goto('/docs/getting-started');await expect(page.locator('article h1')).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(391);});

test('a delayed old guide cannot replace the current guide',async({page})=>{
 let release!:()=>void;const gate=new Promise<void>(r=>release=r);let started=false;
 await page.route('**/assets/templates-*.js',async route=>{started=true;await gate;await route.continue();});
 await page.goto('/docs/getting-started');await expect(page.locator('article h1')).toBeVisible();
 await page.getByRole('link',{name:'Templates',exact:true}).first().click();await expect.poll(()=>started).toBe(true);
 await page.getByRole('link',{name:'Users',exact:true}).first().click();await expect(page.locator('article h1')).toHaveText('Users and subscribers');release();await page.waitForLoadState('networkidle');await expect(page.locator('article h1')).toHaveText('Users and subscribers');
});
