// Keep file bytes in page memory only. Only visible tiles are queued, with two
// thumbnail requests/renders at a time; changing folders cancels the old queue.
export function createFileTiles(root,{storageKey='workservice.cloud.layout'}={}){
  let layout='list',generation=0,active=0,queue=[],controllers=new Set(),observer=null;
  const cache=new Map(),buttons=[...root.querySelectorAll('[data-cloud-layout]')];
  try{if(localStorage.getItem(storageKey)==='tiles')layout='tiles';}catch{}
  const read=node=>({url:node.dataset.url,name:node.dataset.name||'',mime:node.dataset.mime||'',kind:node.dataset.kind});
  function display(node,url){if(!node.isConnected)return;const img=document.createElement('img');img.alt='';img.decoding='async';img.src=url;node.replaceChildren(img);node.classList.remove('is-loading');node.classList.add('has-thumbnail');}
  async function thumbnail(item,signal){
    if(item.mime==='application/pdf'){const {firstPageThumbnail}=await import('/static/js/cloud-pdf.mjs?v=3');return firstPageThumbnail(item.url+'?preview=1',signal);}
    const deadline=Date.now()+45000;
    while(Date.now()<deadline){
      const response=await fetch(item.url+'?preview=thumbnail',{signal,cache:'no-store'});
      if(response.ok&&response.headers.get('content-type')?.startsWith('image/'))return response.blob();
      if(response.status!==202&&response.status!==429)throw new Error('Thumbnail unavailable');
      await new Promise((resolve,reject)=>{const abort=()=>{clearTimeout(timer);reject(new DOMException('Aborted','AbortError'));};const timer=setTimeout(()=>{signal.removeEventListener('abort',abort);resolve();},500);signal.addEventListener('abort',abort,{once:true});if(signal.aborted)abort();});
    }
    throw new Error('Thumbnail timeout');
  }
  function pump(){
    while(active<2&&queue.length){
      const {node,item,serial}=queue.shift();if(serial!==generation||!node.isConnected)continue;
      const controller=new AbortController(),timeout=setTimeout(()=>controller.abort(),45000);controllers.add(controller);active++;
      thumbnail(item,controller.signal).then(blob=>{
        if(controller.signal.aborted||serial!==generation)return;
        const url=URL.createObjectURL(blob);if(cache.has(item.url))URL.revokeObjectURL(cache.get(item.url));cache.set(item.url,url);
        while(cache.size>128){const first=cache.keys().next().value;URL.revokeObjectURL(cache.get(first));cache.delete(first);}
        display(node,url);
      }).catch(()=>{if(serial===generation&&node.isConnected){node.classList.remove('is-loading');node.title='Миниатюра недоступна';}}).finally(()=>{clearTimeout(timeout);active--;controllers.delete(controller);pump();});
    }
  }
  function reset(){generation++;observer?.disconnect();queue=[];for(const controller of controllers)controller.abort();}
  function refresh(){
    reset();root.classList.toggle('cloud-layout-tiles',layout==='tiles');root.dataset.cloudLayout=layout;
    for(const button of buttons)button.setAttribute('aria-pressed',String(button.dataset.cloudLayout===layout));
    const all=root.querySelector('.cloud-list-heading label span');if(all)all.textContent=layout==='tiles'?'Выбрать все':'Имя';
    if(layout!=='tiles')return;
    observer=new IntersectionObserver(entries=>{
      for(const entry of entries)if(entry.isIntersecting){const node=entry.target;observer.unobserve(node);const item=read(node);if(cache.has(item.url))display(node,cache.get(item.url));else{node.classList.add('is-loading');queue.push({node,item,serial:generation});}}
      pump();
    },{rootMargin:'200px'});
    for(const node of root.querySelectorAll('.cloud-thumbnail')){
      const item=read(node);if(item.kind!=='file'||node.dataset.deleted==='true')continue;
      if(['image/jpeg','image/png','image/gif','image/webp','application/pdf'].includes(item.mime)||item.mime.startsWith('video/')||/\.mov$/i.test(item.name))observer.observe(node);
    }
  }
  for(const button of buttons)button.addEventListener('click',()=>{layout=button.dataset.cloudLayout;try{localStorage.setItem(storageKey,layout);}catch{}refresh();});
  refresh();
  return {refresh,destroy(){reset();for(const url of cache.values())URL.revokeObjectURL(url);cache.clear();}};
}
