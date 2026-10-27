const make=(tag,cls,text)=>{const node=document.createElement(tag);if(cls)node.className=cls;if(text)node.textContent=text;return node;};
const command=(label,icon,action)=>{const b=make('button','cl-button');b.type='button';b.title=label;b.setAttribute('aria-label',label);const i=make('span',`cl-icon cl-${icon}`);i.setAttribute('aria-hidden','true');b.append(i);b.addEventListener('click',action);return b;};

export function mountPreview(host,{name,mime,url}){
 let closed=false,cleanup=null,media=null,timer=null,converted=false;
 const controller=new AbortController();
 const status=text=>{if(closed)return;const p=make('p','cloud-preview-status',text);p.setAttribute('role','status');host.replaceChildren(p);};
 const fail=text=>status(text||'Не удалось открыть файл. Оригинал можно скачать.');
 async function webVideo(){
  if(closed)return;converted=true;status('Подготавливаем видео для просмотра…');
  try{
   const response=await fetch(url+'?preview=video',{signal:controller.signal,cache:'no-store'}),data=await response.json();
   if(closed)return;
   if(data.state==='ready'){player('video',url+'?preview=stream');return;}
   if(response.status===202||response.status===429){status(data.state==='busy'?'Сервер обрабатывает другое видео. Ожидание…':'Подготавливаем видео для просмотра…');timer=setTimeout(webVideo,2000);return;}
   fail(data.message||data.error);
  }catch(error){if(!closed&&error.name!=='AbortError')fail('Не удалось загрузить видео. Проверьте соединение и откройте файл снова.');}
 }
 function player(kind,src){
  if(closed)return;media=make(kind);media.controls=true;media.preload='metadata';media.playsInline=true;media.setAttribute('aria-label',name);
  media.addEventListener('error',()=>{if(kind==='video'&&!converted)webVideo();else fail('Не удалось воспроизвести файл. Оригинал можно скачать.');},{once:true});
  media.src=src;host.replaceChildren(media);
 }
 async function start(){
  status('Загрузка…');
  if(mime?.startsWith('text/plain')){
   try{const response=await fetch(url+'?preview=text',{signal:controller.signal,cache:'no-store'}),data=await response.json();if(closed)return;if(!response.ok)throw new Error(data.error);host.replaceChildren(make('pre','cloud-text-preview',data.text));if(data.truncated)host.append(make('p','','Показаны первые 512 КБ'));}catch(error){if(error.name!=='AbortError')fail(error.message);}return;
  }
  if(mime==='application/pdf'){
   try{const {mountPDF}=await import('/static/js/cloud-pdf.mjs?v=2');if(!closed)cleanup=mountPDF(host,url+'?preview=1');}catch{fail('Не удалось открыть PDF. Оригинал можно скачать.');}return;
  }
  if(['image/jpeg','image/png','image/gif','image/webp'].includes(mime)){
   let zoom=1;const shell=make('div','cloud-image-preview'),tools=make('div','cloud-image-tools'),stage=make('div','cloud-image-stage'),value=make('output','','100%'),image=make('img');image.alt=name;
   const render=()=>{if(closed||!image.naturalWidth)return;const fit=Math.min(image.naturalWidth,Math.max(120,stage.clientWidth-20),Math.max(150,innerHeight*.62)/image.naturalHeight*image.naturalWidth);image.style.width=fit*zoom+'px';value.textContent=Math.round(zoom*100)+'%';};
   tools.append(command('Уменьшить','zoom-out',()=>{zoom=Math.max(.5,zoom-.25);render();}),value,command('Увеличить','zoom-in',()=>{zoom=Math.min(3,zoom+.25);render();}),command('По размеру окна','fit',()=>{zoom=1;render();}));
   image.addEventListener('load',render);image.addEventListener('error',()=>fail('Не удалось открыть изображение. Оригинал можно скачать.'));image.src=url+'?preview=1';stage.tabIndex=0;stage.setAttribute('role','region');stage.setAttribute('aria-label','Изображение: '+name);stage.append(image);shell.append(tools,stage);host.replaceChildren(shell);
   const observer=new ResizeObserver(render);observer.observe(stage);cleanup=()=>observer.disconnect();return;
  }
  if(mime?.startsWith('video/')||/\.mov$/i.test(name)){if(mime==='video/quicktime'||/\.mov$/i.test(name))webVideo();else player('video',url+'?preview=1');return;}
  if(mime?.startsWith('audio/')){player('audio',url+'?preview=1');return;}
  fail('Для этого формата предпросмотр недоступен. Оригинал можно скачать.');
 }
 start();
 return()=>{closed=true;clearTimeout(timer);controller.abort();cleanup?.();if(media){media.pause();media.removeAttribute('src');media.load();}};
}
