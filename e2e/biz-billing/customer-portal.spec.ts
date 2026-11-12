import { test,expect } from './fixtures';

const token='pt2_customer-portal-fixture';
async function portalFixture(page:import('@playwright/test').Page,options:{partial?:boolean;delayed?:boolean;expired?:boolean;cancel?:boolean;sdkFailure?:boolean}={}){
  const privateRequests:string[]=[],headers:Record<string,string>[]=[],writes:{path:string;key:string}[]=[],listLimits:number[]=[];
  let expired=!!options.expired,captured=false;
  const invoice={id:'invoice-1',app_id:'app-private',user_id:'customer-1',invoice_number:'INV-000001',currency:'INR',status:options.partial?'partially_paid':'open',total_paisa:1180,amount_paid_paisa:options.partial?590:0,amount_due_paisa:options.partial?590:1180,credits_applied:0,cash_refunded_paisa:0,collection_owner:'',gateway_mode:'test',created_at:'2026-11-12T00:00:00Z'};
  const subscription={id:'subscription-1',user_id:'customer-1',plan_name:'Care membership',currency:'INR',plan_amount_paisa:1000,collection_mode:'automatic',collection_state:'awaiting_authorization',status:'awaiting_authorization',gateway_mode:'test',current_period_end:'0001-01-01T00:00:00Z'};
  const document=()=>({...invoice,invoice_id:invoice.id,issuer:{name:'Frozen Clinic',email:'clinic@example.invalid',address:{line1:'12 Main Road',city:'Pune',state:'MH',pincode:'411001',country:'India'},gstin:'FIXTURE-GST'},customer:{name:'Fixture Patient',email:'patient@example.invalid',address:{city:'Pune'}},line_items:[{description:'Consultation <script>window.compromised=true</script>',quantity:1,unit_price:1000,amount_paisa:1000,tax_rate:18}],subtotal_paisa:1000,discount_paisa:0,tax_amount_paisa:180,tax_breakdown:{igst:180},issued_at:'2026-11-12T00:00:00Z',due_date:'2026-11-20T00:00:00Z',payment_terms:'due_on_receipt',notes:'Thank you',footer_text:'Frozen invoice footer',payments:invoice.amount_paid_paisa?[{id:'payment-1',method:'gateway',reference:'RECEIPT-001',amount_paisa:invoice.amount_paid_paisa,refunded_paisa:invoice.cash_refunded_paisa,status:'success',created_at:'2026-11-12T00:00:00Z',refunds:invoice.cash_refunded_paisa?[{id:'refund-1',reference:'REFUND-001',amount_paisa:invoice.cash_refunded_paisa,state:'completed'}]:[]}]:[],credits:[]});
  await page.addInitScript(({cancel,sdkFailure})=>{if(sdkFailure)return;(window as any).Razorpay=class{options:any;constructor(options:any){this.options=options;(window as any).__checkout=options;}on(){}open(){if(cancel)this.options.modal.ondismiss();}};},{cancel:options.cancel,sdkFailure:options.sdkFailure});
  if(options.sdkFailure)await page.route('https://checkout.razorpay.com/**',route=>route.abort());
  await page.route('**/v1/**',async route=>{
    const request=route.request(),url=new URL(request.url());headers.push(request.headers());
    if(!url.pathname.startsWith(`/v1/portal/${token}`)){privateRequests.push(url.pathname);return route.fulfill({status:403,json:{error:'Private dashboard API forbidden'}});}
    if(expired)return route.fulfill({status:401,json:{error:'Portal link expired'}});
    const path=url.pathname.replace(`/v1/portal/${token}`,'').replace(/\/$/,'');
    const reply=(data:unknown)=>route.fulfill({json:{success:true,data}});
    if(request.method()!=='GET')writes.push({path,key:request.headers()['idempotency-key']??''});
    if(path==='/invoices'){listLimits.push(Number(url.searchParams.get('limit')??50));return reply([invoice]);}
    if(path==='/subscriptions'){listLimits.push(Number(url.searchParams.get('limit')??20));return reply([subscription]);}
    if(path==='/invoices/invoice-1')return reply(invoice);
    if(path==='/invoices/invoice-1/document')return reply(document());
    if(path==='/subscriptions/subscription-1')return reply(subscription);
    if(path==='/invoices/invoice-1/pay')return reply({operation_id:'order-operation',gateway_order_id:'order-fixture',key_id:'rzp_test_public',amount_due_paisa:invoice.amount_due_paisa,currency:'INR',invoice_number:invoice.invoice_number,state:'ready'});
    if(path==='/invoices/invoice-1/verify-payment'){
      if(options.delayed&&!captured)return route.fulfill({status:503,json:{code:'BILLING_GATEWAY_PENDING',operation_id:'capture-operation',retryable:true}});
      invoice.status='paid';invoice.amount_paid_paisa=1180;invoice.amount_due_paisa=0;captured=true;return reply({operation_id:'capture-operation',state:'completed'});
    }
    if(path.startsWith('/operations/'))return reply({operation_id:'capture-operation',state:captured?'completed':'pending'});
    if(path==='/subscriptions/subscription-1/authorize')return reply({operation_id:'authorize-operation',gateway_subscription_id:'sub-fixture',key_id:'rzp_test_public',state:'ready',currency:'INR'});
    if(path==='/subscriptions/subscription-1/verify-authorization'){subscription.status='active';subscription.collection_state='authorized';return reply({operation_id:'authorize-operation',state:'authorized'});}
    return route.fulfill({status:404,json:{error:'Not found'}});
  });
  return {invoice,subscription,writes,privateRequests,headers,listLimits,expire:()=>{expired=true;},capture:()=>{captured=true;invoice.status='paid';invoice.amount_paid_paisa=1180;invoice.amount_due_paisa=0;}};
}

test('public customer portal needs no dashboard session or private headers',async({page})=>{
  const fixture=await portalFixture(page);const response=await page.goto(`/portal/${token}`);
  expect(response?.headers()['referrer-policy']).toBe('no-referrer');
  expect(response?.headers()['cache-control']).toContain('no-store');
  await expect(page.getByRole('heading',{name:'Your billing',exact:true})).toBeVisible();
  await expect(page.getByRole('link',{name:/INV-000001/})).toBeVisible();
  await expect(page.getByRole('button',{name:'Manage merchant credentials'})).toHaveCount(0);
  expect(fixture.privateRequests).toEqual([]);expect(fixture.headers.every(h=>!h.authorization&&!h['x-api-key'])).toBe(true);
  expect(fixture.listLimits.every(limit=>limit===20)).toBe(true);
  expect(await page.evaluate(token=>JSON.stringify(localStorage).includes(token),token)).toBe(false);
});

for(const partial of [false,true])test(`verified ${partial?'partial balance':'full'} invoice payment survives reload`,async({page})=>{
  const fixture=await portalFixture(page,{partial});await page.goto(`/portal/${token}/invoices/invoice-1`);
  await page.getByRole('button',{name:'Pay invoice',exact:true}).click();
  await page.waitForFunction(()=>!!(window as any).__checkout);await page.evaluate(()=>{(window as any).__checkout.handler({razorpay_order_id:'order-fixture',razorpay_payment_id:'payment-fixture',razorpay_signature:'fixture-valid'});});
  await expect(page.getByText('Payment confirmed',{exact:true})).toBeVisible();
  await expect(page.getByTestId('portal-amount-due')).toHaveText('INR 0.00');
  await page.reload();await expect(page.getByTestId('portal-amount-due')).toHaveText('INR 0.00');
  expect(fixture.writes.filter(w=>w.path.endsWith('/pay')).every(w=>!!w.key)).toBe(true);
});

test('delayed capture stays pending until the server confirms it',async({page})=>{
  const fixture=await portalFixture(page,{delayed:true});await page.goto(`/portal/${token}/invoices/invoice-1`);
  await page.getByRole('button',{name:'Pay invoice',exact:true}).click();await page.waitForFunction(()=>!!(window as any).__checkout);await page.evaluate(()=>(window as any).__checkout.handler({razorpay_order_id:'order-fixture',razorpay_payment_id:'payment-fixture',razorpay_signature:'fixture-valid'}));
  await expect(page.getByText('Awaiting payment confirmation',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Pay invoice',exact:true})).toBeDisabled();fixture.capture();
  await page.getByRole('button',{name:'Check payment status',exact:true}).click();
  await expect(page.getByText('Payment confirmed',{exact:true})).toBeVisible();
});

test('canceled checkout restores focus and does not record payment',async({page})=>{
  const fixture=await portalFixture(page,{cancel:true});await page.goto(`/portal/${token}/invoices/invoice-1`);
  const pay=page.getByRole('button',{name:'Pay invoice',exact:true});await pay.click();
  await expect(page.getByText('Checkout closed. No payment has been confirmed.',{exact:true})).toBeVisible();await expect(pay).toBeFocused();expect(fixture.invoice.amount_paid_paisa).toBe(0);
});

test('mandate authorization requires explicit consent and does not claim invoice payment',async({page})=>{
  const fixture=await portalFixture(page);await page.goto(`/portal/${token}/subscriptions/subscription-1`);
  await expect(page.getByRole('button',{name:'Authorize automatic payments',exact:true})).toBeDisabled();
  await page.getByLabel('I authorize the recurring payment arrangement',{exact:true}).check();
  await page.getByRole('button',{name:'Authorize automatic payments',exact:true}).click();
  await page.waitForFunction(()=>!!(window as any).__checkout);await page.evaluate(()=>(window as any).__checkout.handler({razorpay_subscription_id:'sub-fixture',razorpay_payment_id:'payment-fixture',razorpay_signature:'fixture-valid'}));
  await expect(page.getByText('Authorization confirmed',{exact:true})).toBeVisible();expect(fixture.invoice.amount_paid_paisa).toBe(0);
});

test('an account still awaiting verification cannot be reported as authorized',async({page})=>{
  await portalFixture(page);let checked=0;
  await page.route('**/subscriptions/subscription-1/verify-authorization',route=>route.fulfill({json:{success:true,data:{operation_id:'authorize-operation',state:'awaiting_account_verification'}}}));
  await page.route('**/operations/authorize-operation',route=>{checked++;return route.fulfill({json:{success:true,data:{operation_id:'authorize-operation',state:'provider_created',kind:'recurring',result:{state:'completed'}}}});});
  await page.goto(`/portal/${token}/subscriptions/subscription-1`);
  await page.getByLabel('I authorize the recurring payment arrangement',{exact:true}).check();await page.getByRole('button',{name:'Authorize automatic payments',exact:true}).click();
  await page.waitForFunction(()=>!!(window as any).__checkout);await page.evaluate(()=>(window as any).__checkout.handler({razorpay_subscription_id:'sub-fixture',razorpay_payment_id:'payment-fixture',razorpay_signature:'fixture-valid'}));
  await page.getByRole('button',{name:'Check authorization status',exact:true}).click();await expect.poll(()=>checked).toBeGreaterThan(0);
  await expect(page.getByText('Awaiting authorization confirmation',{exact:true})).toBeVisible();await expect(page.getByText('Authorization confirmed',{exact:true})).toHaveCount(0);
});

test('expired link after checkout does not regain access when callback settles payment',async({page})=>{
  const fixture=await portalFixture(page);await page.goto(`/portal/${token}/invoices/invoice-1`);await page.getByRole('button',{name:'Pay invoice',exact:true}).click();fixture.expire();fixture.capture();
  await page.waitForFunction(()=>!!(window as any).__checkout);await page.evaluate(()=>(window as any).__checkout.handler({razorpay_order_id:'order-fixture',razorpay_payment_id:'payment-fixture',razorpay_signature:'fixture-valid'}));
  await expect(page.getByText(/This billing link has expired or been revoked/)).toBeVisible();expect(fixture.invoice.status).toBe('paid');await expect(page).toHaveURL(new RegExp(`/portal/${token}`));
});

for(const width of [360,390,768,1280])for(const dark of [false,true])test(`issued invoice print layout ${width}px ${dark?'dark':'light'}`,async({page},testInfo)=>{
  await page.setViewportSize({width,height:900});const fixture=await portalFixture(page);fixture.capture();fixture.invoice.cash_refunded_paisa=300;
  await page.goto(`/portal/${token}/invoices/invoice-1/print`);if(dark)await page.evaluate(()=>document.documentElement.classList.add('dark'));
  await expect(page.getByText('Frozen Clinic',{exact:true})).toBeVisible();await expect(page.getByText('Fixture Patient',{exact:true})).toBeVisible();await expect(page.getByText('REFUND-001',{exact:false})).toBeVisible();
  await expect(page.getByText(/Consultation <script>/)).toBeVisible();expect(await page.evaluate(()=>(window as any).compromised)).toBeUndefined();
  const overflow=await page.evaluate(()=>Array.from(document.querySelectorAll('main *')).filter(node=>node.getBoundingClientRect().right>innerWidth+1).map(node=>({tag:node.tagName,className:node.className,width:node.getBoundingClientRect().width})));
  expect(overflow).toEqual([]);
  await page.screenshot({path:testInfo.outputPath('invoice-document.png'),fullPage:true});
  const pdf=await page.pdf({format:'A4',printBackground:true,path:testInfo.outputPath('invoice.pdf')});expect(pdf.subarray(0,4).toString()).toBe('%PDF');
});

test('SDK load failure is recoverable without claiming a charge',async({page})=>{
  const fixture=await portalFixture(page,{sdkFailure:true});await page.goto(`/portal/${token}/invoices/invoice-1`);await page.getByRole('button',{name:'Pay invoice',exact:true}).click();await expect(page.getByRole('alert')).toContainText('Could not load secure checkout');await expect(page.getByRole('button',{name:'Pay invoice',exact:true})).toBeEnabled();expect(fixture.invoice.amount_paid_paisa).toBe(0);
});

test('expired or revoked link shows a customer-safe message without login redirect',async({page})=>{
  await portalFixture(page,{expired:true});await page.goto(`/portal/${token}`);await expect(page.getByText(/This billing link has expired or been revoked/)).toBeVisible();await expect(page).toHaveURL(new RegExp(`/portal/${token}`));
});


test('printing exports only the issued document without the bearer URL',async({page})=>{
 await portalFixture(page);
 await page.addInitScript(()=>{
  window.print=()=>{};
  const original=Document.prototype.createElement;
  Document.prototype.createElement=function(...args:any[]){
   const element=(original as any).apply(this,args);
   if(args[0]==='iframe')element.addEventListener('load',()=>{element.contentWindow.print=()=>{(window as any).__printed={url:element.contentWindow.location.href,html:element.contentDocument.documentElement.outerHTML};};});
   return element;
  } as any;
 });
 await page.goto(`/portal/${token}/invoices/invoice-1/print`);
 await expect(page.getByText('Frozen Clinic',{exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Print / Save as PDF',exact:true}).click();
 await expect.poll(()=>page.evaluate(()=>(window as any).__printed?.url)).toBe('about:srcdoc');
 const html=await page.evaluate(()=>(window as any).__printed.html as string);
 expect(html).toContain('Frozen Clinic');expect(html).not.toContain(token);expect(html).not.toContain('access_token');
});
