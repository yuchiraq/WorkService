(() => {
  'use strict';
  let popup, active, items = [], index = -1, sequence = 0, timer, pointerInside=false;
  const id = 'document-suggestion-list';
  function ensurePopup(){if(!popup){popup=document.createElement('div');popup.id=id;popup.className='docs-suggestion-popup';popup.setAttribute('role','listbox');popup.setAttribute('aria-label','Подсказки');popup.setAttribute('popover','manual');popup.hidden=true;document.body.append(popup);}}
  function close() {
    sequence++;clearTimeout(timer);
    if (active) { active.control.setAttribute('aria-expanded','false');active.input.removeAttribute('aria-activedescendant'); }
    if (popup) { if (popup.matches(':popover-open')) popup.hidePopover();popup.hidden=true; }
    active=null;items=[];index=-1;
  }
  function position() {
    if (!active||popup.hidden) return;
    const rect=active.input.getBoundingClientRect(),vv=window.visualViewport;
    const topBound=vv?.offsetTop||0,height=(vv?.height||innerHeight)+topBound;
    const width=Math.min(Math.max(rect.width,280),innerWidth-16),availableBelow=height-rect.bottom-8,above=availableBelow<150&&rect.top-topBound>availableBelow;
    const maxHeight=Math.max(90,Math.min(260,above?rect.top-topBound-12:availableBelow));
    popup.style.width=width+'px';popup.style.maxHeight=maxHeight+'px';
    popup.style.left=Math.max(8,Math.min(rect.left,innerWidth-width-8))+'px';
    popup.style.top=(above?Math.max(topBound+8,rect.top-Math.min(popup.scrollHeight,maxHeight)-4):rect.bottom+4)+'px';
  }
  function choose(i) {
    if (!active||!items[i])return;const current=active,item=items[i];close();
    current.applying=true;current.select(item);current.applying=false;clearTimeout(timer);sequence++;
    if(document.activeElement!==current.input){current.skipFocus=true;current.input.focus({preventScroll:true});}
  }
  function highlight() {
    [...popup.children].forEach((node,i)=>node.setAttribute('aria-selected',String(i===index)));
    if (index>=0) { active.input.setAttribute('aria-activedescendant',id+'-'+index);popup.children[index].scrollIntoView({block:'nearest'}); }
  }
  async function show(config) {
    const request=++sequence;
    try {
      const result=await config.items(config.input.value);
      if (request!==sequence||!config.input.isConnected||document.activeElement!==config.input)return;
      if (!result.length){close();return;}
      ensurePopup();
      if(popup.matches(':popover-open'))popup.hidePopover();
      const host=config.input.closest('dialog')||document.body;host.append(popup);popup.replaceChildren();
      active=config;items=result.slice(0,12);index=-1;
      items.forEach((item,i)=>{
        const option=document.createElement('div');option.id=id+'-'+i;option.setAttribute('role','option');option.setAttribute('aria-selected','false');option.className='docs-suggestion-option';
        const label=document.createElement('span');label.textContent=item.label||item.value;option.append(label);
        if(item.detail){const detail=document.createElement('small');detail.textContent=item.detail;option.append(detail);}
        option.addEventListener('pointerdown',event=>{if(event.pointerType==='mouse')event.preventDefault();});
        option.addEventListener('click',()=>choose(i));popup.append(option);
      });
      popup.hidden=false;if(popup.showPopover)popup.showPopover();config.control.setAttribute('aria-expanded','true');position();
    } catch { if(request===sequence)close(); }
  }
  function attach(input,options) {
    if(input.dataset.documentSuggestions)return;input.dataset.documentSuggestions='true';
    ensurePopup();let control=input;
    if(input.tagName==='TEXTAREA'){control=document.createElement('div');control.className='docs-text-combobox';input.before(control);control.append(input);control.setAttribute('aria-label',options.label||input.getAttribute('aria-label')||'Поле документа');}
    control.setAttribute('role','combobox');control.setAttribute('aria-haspopup','listbox');control.setAttribute('aria-controls',id);control.setAttribute('aria-expanded','false');input.setAttribute('aria-autocomplete','list');input.setAttribute('aria-controls',id);input.autocomplete='off';
    const config={input,control,...options};
    input.addEventListener('input',()=>{if(config.applying)return;clearTimeout(timer);sequence++;timer=setTimeout(()=>show(config),180);});
    input.addEventListener('focus',()=>{if(config.skipFocus){config.skipFocus=false;return;}show(config);});
    input.addEventListener('blur',()=>{if(active?.input===input&&!pointerInside)close();});
    input.addEventListener('keydown',event=>{
      if(active?.input!==input)return;
      if(event.key==='ArrowDown'||event.key==='ArrowUp'){event.preventDefault();index=(index+(event.key==='ArrowDown'?1:-1)+items.length)%items.length;highlight();}
      else if(event.key==='Enter'&&index>=0){event.preventDefault();choose(index);}
      else if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close();}
      else if(event.key==='Tab')close();
    });
  }
  document.addEventListener('pointerdown',event=>{pointerInside=!!active&&popup.contains(event.target);if(active&&!pointerInside&&event.target!==active.input)close();},true);
  // Touch generates a compatibility mousedown after pointerup; retain the blur
  // guard until click so the option is not removed before selection.
  document.addEventListener('click',()=>{pointerInside=false;},true);
  document.addEventListener('pointercancel',()=>{pointerInside=false;if(active&&document.activeElement!==active.input)close();},true);
  window.addEventListener('resize',position);window.visualViewport?.addEventListener('resize',position);
  document.addEventListener('scroll',position,true);
  window.WorkServiceDocumentSuggestions={attach,close};
})();
