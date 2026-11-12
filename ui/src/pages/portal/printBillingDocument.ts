// Print an isolated issued document. Printing the bearer-link page directly
// would let browser headers/footers put the private URL into a shared PDF.
export function printBillingDocument(article:HTMLElement){
  const previous=document.activeElement as HTMLElement|null;
  const frame=document.createElement('iframe');frame.title='Invoice print preview';frame.referrerPolicy='no-referrer';
  frame.style.cssText='position:fixed;width:1px;height:1px;left:-10000px;top:0;border:0';
  let cleanupTimer:ReturnType<typeof setTimeout>;
  const cleanup=()=>{clearTimeout(cleanupTimer);frame.remove();previous?.focus();};
  frame.onload=()=>{
    const target=frame.contentDocument,view=frame.contentWindow;if(!target||!view){cleanup();return;}
    target.body.appendChild(target.importNode(article,true));
    view.addEventListener('afterprint',cleanup,{once:true});
    view.focus();view.print();
  };
  // The only copied HTML comes from React's already escaped invoice document.
  // No links, dashboard session data or token-bearing URL enters this frame.
  frame.srcdoc='<!doctype html><html><head><meta name="referrer" content="no-referrer"><title>Invoice</title><style>@page{size:A4;margin:14mm}body{font:10pt/1.5 Arial,sans-serif;color:#111;background:#fff}h1{font-size:19pt}h2{font-size:12pt}p{margin:3pt 0;overflow-wrap:anywhere}article>header{margin-bottom:14pt}section{margin:12pt 0}.invoice-line{border:1px solid #bbb;padding:8pt;margin:8pt 0;break-inside:avoid}.invoice-totals{max-width:260pt;margin-left:auto;break-inside:avoid}.invoice-totals>div{display:flex;justify-content:space-between;gap:16pt}dd{margin:0;font-weight:bold}footer{border-top:1px solid #bbb;margin-top:14pt;padding-top:8pt}</style></head><body></body></html>';
  cleanupTimer=setTimeout(cleanup,60000);document.body.appendChild(frame);
}
