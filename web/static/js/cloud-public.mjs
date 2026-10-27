import {mountPreview} from '/static/js/cloud-preview.mjs?v=1';
import {createFileTiles} from '/static/js/cloud-tiles.mjs?v=1';
const root=document.querySelector('.cloud-public');
const host=document.getElementById('shared-preview');
let dispose=null;
const tiles=createFileTiles(root,{storageKey:'workservice.cloud.public.layout'});
if(host)dispose=mountPreview(host,{name:host.dataset.name,mime:host.dataset.mime,url:host.dataset.url});
window.addEventListener('pagehide',()=>{dispose?.();tiles.destroy();},{once:true});
window.addEventListener('pageshow',event=>{if(event.persisted){tiles.refresh();if(host)dispose=mountPreview(host,{name:host.dataset.name,mime:host.dataset.mime,url:host.dataset.url});}});
const archive=document.getElementById('shared-download-all'),status=document.getElementById('shared-download-status');
archive?.addEventListener('click',async()=>{
 if(archive.disabled)return;archive.disabled=true;status.textContent='Формируем архив…';
 try{
  const response=await fetch(root.dataset.archive,{method:'POST',headers:{Accept:'application/json'},cache:'no-store'}),data=await response.json();
  if(!response.ok)throw new Error(data.error||'Не удалось создать архив');
  const a=document.createElement('a');a.href=data.url;a.download='';document.body.append(a);a.click();a.remove();status.textContent='Архив готов. Скачивание началось.';
 }catch(error){status.textContent=error.message||'Нет соединения. Повторите скачивание.';}
 finally{archive.disabled=false;}
});
