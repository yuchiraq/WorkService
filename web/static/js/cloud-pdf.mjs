import * as pdfjs from '/static/vendor/pdfjs-6.4.299/pdf.mjs';

const assets='/static/vendor/pdfjs-6.4.299/';
pdfjs.GlobalWorkerOptions.workerSrc=assets+'pdf.worker.mjs';

// Render pixels/text only; never attach PDF actions, scripts or external links.
export function mountPDF(host,url) {
  let closed=false,documentPDF=null,renderTask=null,pageNumber=1,serial=0,timer;
  const make=(tag,className,text)=>{const e=document.createElement(tag);if(className)e.className=className;if(text)e.textContent=text;return e;};
  const shell=make('div','cloud-pdf'),toolbar=make('div','cloud-pdf-toolbar'),stage=make('div','cloud-pdf-stage'),status=make('p','cloud-pdf-status','Загрузка PDF...');
  const command=(name,icon,action)=>{const b=make('button','cl-button');b.type='button';b.title=name;b.setAttribute('aria-label',name);const i=make('span',`cl-icon cl-${icon}`);i.setAttribute('aria-hidden','true');b.append(i);b.addEventListener('click',action);return b;};
  const previous=command('Предыдущая страница','previous',()=>{pageNumber--;render();});
  const next=command('Следующая страница','next',()=>{pageNumber++;render();});
  const number=make('input');number.type='number';number.min='1';number.value='1';number.setAttribute('aria-label','Страница PDF');
  const total=make('span','','/ 1'),zoom=make('select');zoom.setAttribute('aria-label','Масштаб PDF');
  for(const [label,value] of [['По ширине','1'],['125%','1.25'],['150%','1.5'],['200%','2']])zoom.add(new Option(label,value));
  const canvas=make('canvas');canvas.setAttribute('role','img');
  const textDetails=make('details','cloud-pdf-text'),summary=make('summary','','Текст страницы'),text=make('pre');textDetails.append(summary,text);
  status.setAttribute('role','status');toolbar.append(previous,number,total,next,zoom);stage.append(canvas);shell.append(toolbar,status,stage,textDetails);host.replaceChildren(shell);
  const loading=pdfjs.getDocument({url,cMapUrl:assets+'cmaps/',cMapPacked:true,standardFontDataUrl:assets+'standard_fonts/',wasmUrl:assets+'wasm/',isEvalSupported:false,disableAutoFetch:true,disableStream:true,maxImageSize:32*1024*1024});
  let previousWidth=host.clientWidth;
  const observer=new ResizeObserver(()=>{const width=host.clientWidth;if(Math.abs(width-previousWidth)<1)return;previousWidth=width;clearTimeout(timer);timer=setTimeout(()=>{if(documentPDF&&!closed)render();},120);});observer.observe(host);
  const controls=()=>{previous.disabled=!documentPDF||pageNumber<=1;next.disabled=!documentPDF||pageNumber>=documentPDF.numPages;number.disabled=zoom.disabled=!documentPDF;number.value=String(pageNumber);};
  async function render() {
    if(closed||!documentPDF)return;
    const request=++serial;controls();renderTask?.cancel();status.hidden=false;status.textContent='Загрузка страницы...';text.textContent='';
    try {
      const page=await documentPDF.getPage(pageNumber);if(closed||request!==serial)return;
      const viewport=page.getViewport({scale:1});const width=Math.max(180,stage.clientWidth-16)*Number(zoom.value);
      const scale=width/viewport.width;const view=page.getViewport({scale});
      const ratio=Math.min(devicePixelRatio||1,2,Math.sqrt(12*1024*1024/(view.width*view.height)));
      canvas.width=Math.ceil(view.width*ratio);canvas.height=Math.ceil(view.height*ratio);canvas.style.width=view.width+'px';canvas.style.height=view.height+'px';canvas.setAttribute('aria-label',`Страница ${pageNumber} из ${documentPDF.numPages}`);
      renderTask=page.render({canvasContext:canvas.getContext('2d'),viewport:view,transform:[ratio,0,0,ratio,0,0],background:'#ffffff'});await renderTask.promise;
      if(closed||request!==serial)return;
      status.hidden=true;
      const content=await page.getTextContent();if(closed||request!==serial)return;
      text.textContent=content.items.map(item=>item.str+(item.hasEOL?'\n':' ')).join('').slice(0,200000)||'На этой странице нет текстового слоя';
    }catch(error){if(closed||request!==serial||error.name==='RenderingCancelledException')return;status.hidden=false;status.textContent='Не удалось отобразить страницу. Файл можно скачать';}
  }
  number.addEventListener('change',()=>{pageNumber=Math.max(1,Math.min(documentPDF.numPages,Number.parseInt(number.value,10)||1));render();});
  zoom.addEventListener('change',render);controls();
  loading.promise.then(doc=>{if(closed){doc.destroy();return;}documentPDF=doc;number.max=String(doc.numPages);total.textContent=`/ ${doc.numPages}`;render();}).catch(error=>{if(closed)return;status.textContent=error.name==='PasswordException'?'PDF защищён паролем. Скачайте его для открытия':'Не удалось открыть PDF. Файл можно скачать';});
  return ()=>{closed=true;serial++;clearTimeout(timer);observer.disconnect();renderTask?.cancel();loading.destroy().catch(()=>{});};
}
