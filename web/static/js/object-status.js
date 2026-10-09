(() => {
  'use strict';
  document.querySelectorAll('[data-object-status-form]:not([data-status-ready])').forEach(form => {
    form.dataset.statusReady = 'true';
    if (!form.dataset.objectId) return;
    const select = form.querySelector('[name=status]');
    const confirmed = document.createElement('input');
    confirmed.type = 'hidden';
    confirmed.name = 'inventory_confirmed_status';
    form.append(confirmed);
    const dialog = document.createElement('dialog');
    const titleID = 'inventory-warning-' + Math.random().toString(36).slice(2);
    dialog.className = 'confirmation-dialog object-inventory-warning';
    dialog.setAttribute('aria-labelledby',titleID);
    dialog.innerHTML = '<div class="modal-content"><h2 id="'+titleID+'">На объекте остался инвентарь</h2><p class="inventory-status-warning"></p><ul class="inventory-status-summary"></ul><p>Инвентарь останется закреплён за объектом.</p><div class="form-actions"><button type="button" class="btn btn-secondary" data-cancel autofocus>Отмена</button><button type="button" class="btn btn-primary" data-confirm>Изменить статус</button></div></div>';
    form.after(dialog);
    window.WorkServiceUI?.prepare(dialog.parentElement);
    let pending = false;
    let approved = false;
    let submitter;
    let targetStatus;
    const labels = {pcs:'шт.',set:'компл.',pack:'упак.',m:'м',m2:'м²',m3:'м³',kg:'кг',l:'л'};
    dialog.querySelector('[data-cancel]').addEventListener('click',()=>dialog.close());
    dialog.querySelector('[data-confirm]').addEventListener('click',()=>{
      if (select.value !== targetStatus) { dialog.close(); return; }
      confirmed.value = targetStatus;
      approved = true;
      dialog.close();
      form.requestSubmit(submitter);
    });
    select.addEventListener('change',()=>{confirmed.value='';approved=false;});
    form.addEventListener('submit',async event=>{
      if (approved) { approved=false; return; }
      if (select.value === form.dataset.originalStatus || !['paused','completed'].includes(select.value)) return;
      event.preventDefault();
      if (pending) return;
      pending=true;
      submitter=event.submitter || form.querySelector('[type=submit]');
      targetStatus=select.value;
      confirmed.value='';
      window.WorkServiceUI?.beginSubmission(form,submitter);
      const controller = new AbortController();
      const timeout = setTimeout(()=>controller.abort(),15000);
      let shouldSubmit=false;
      try {
        const response=await fetch('/object/'+encodeURIComponent(form.dataset.objectId)+'/inventory',{cache:'no-store',signal:controller.signal,headers:{Accept:'application/json'}});
        if (!response.ok || response.redirected) throw new Error('inventory');
        const state=await response.json();
        if (!form.isConnected || targetStatus!==select.value) return;
        const items=state.items.filter(item=>item.quantity>0);
        if (items.length) {
          dialog.querySelector('.inventory-status-warning').textContent='Позиций с остатком: '+items.length+'. '+(targetStatus==='paused'?'Поставить объект на паузу?':'Закрыть объект?');
          const list=dialog.querySelector('ul');
          list.replaceChildren(...items.slice(0,8).map(item=>{
            const li=document.createElement('li');
            li.textContent=item.name+' - '+(item.quantity/1000).toLocaleString('ru-RU',{maximumFractionDigits:3})+' '+labels[item.unit];
            return li;
          }));
          if(items.length>8){const li=document.createElement('li');li.textContent='Ещё позиций: '+(items.length-8);list.append(li);}
          dialog.uiReturnFocus=submitter;
          dialog.showModal();
        } else { approved=true;shouldSubmit=true; }
      } catch (_) {
        window.WorkServiceUI?.feedback('Не удалось проверить остатки инвентаря. Статус не изменён. Повторите попытку.');
      } finally {
        clearTimeout(timeout);
        pending=false;
        window.WorkServiceUI?.endSubmission(form);
      }
      if(shouldSubmit) form.requestSubmit(submitter);
    });
  });
})();
