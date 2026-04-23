package api

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type navItem struct {
	PageID string
	Path   string
	Label  string
}

// RenderSidebar keeps historical name, but renders topbar with off-canvas navigation.
func RenderSidebar(c *gin.Context, activePage string) string {
	userName := "Пользователь"
	if userNameValue, ok := c.Get("userName"); ok {
		if value, castOK := userNameValue.(string); castOK && strings.TrimSpace(value) != "" {
			userName = value
		}
	}

	userStatus := "user"
	if userStatusValue, ok := c.Get("userStatus"); ok {
		if value, castOK := userStatusValue.(string); castOK && strings.TrimSpace(value) != "" {
			userStatus = value
		}
	}

	navItems := []navItem{
		{PageID: "schedule", Path: "/schedule", Label: "Расписание"},
		{PageID: "timesheets", Path: "/timesheets", Label: "Табель"},
		{PageID: "improvements", Path: "/improvements", Label: "Улучшения/ошибки"},
	}
	if userStatus == "admin" {
		navItems = append([]navItem{
			{PageID: "dashboard", Path: "/dashboard", Label: "Панель"},
			{PageID: "workers", Path: "/workers", Label: "Работники"},
			{PageID: "objects", Path: "/objects", Label: "Объекты"},
		}, navItems...)
		navItems = append(navItems,
			navItem{PageID: "users", Path: "/users", Label: "Пользователи"},
			navItem{PageID: "settings", Path: "/settings", Label: "Настройки"},
		)
	}

	var nav strings.Builder
	pageTitle := "Раздел"
	for _, item := range navItems {
		class := "nav-link"
		if item.PageID == activePage {
			class += " active"
			pageTitle = item.Label
		}
		nav.WriteString(fmt.Sprintf(`<a class="%s" href="%s">%s</a>`, class, item.Path, item.Label))
	}
	if activePage == "my-profile" {
		pageTitle = "Мой профиль"
	}

	roleLabel := "Пользователь"
	if userStatus == "admin" {
		roleLabel = "Администратор"
	}

	userInitial := ""
	if utf8.RuneCountInString(userName) > 0 {
		firstRune, _ := utf8.DecodeRuneInString(userName)
		userInitial = strings.ToUpper(string(firstRune))
	}

	csrfToken, _ := c.Get("csrfToken")
	csrfScript := ""
	if token, ok := csrfToken.(string); ok && token != "" {
		csrfScript = fmt.Sprintf(`<script>(function(){const t=%q;document.querySelectorAll('form[method="POST"], form[method="post"]').forEach(function(f){if(f.querySelector('input[name="_csrf_token"]')) return; var i=document.createElement('input'); i.type='hidden'; i.name='_csrf_token'; i.value=t; f.appendChild(i);});})();</script>`, token)
	}

	uiScript := `<script>(function(){
const body=document.body;
const modal=document.getElementById('app-action-modal');
const modalBody=modal ? modal.querySelector('.action-modal-body') : null;
const modalContent=document.getElementById('app-action-modal-content');
const closeBtn=document.querySelector('[data-modal-close]');
const modalTitle=document.getElementById('app-action-modal-title');
const burger=document.querySelector('[data-mobile-nav-toggle]');
const navOverlay=document.querySelector('[data-nav-overlay]');
const parser=new DOMParser();
const defaultModalTitle='Форма';
let modalRequestSeq=0;
let modalCleanupFns=[];
let lastModalTrigger=null;
let tooltipPortal=null;
let activeHoursCell=null;
let tooltipHideTimer=0;

function syncNavState(){
  if(!burger) return;
  burger.setAttribute('aria-expanded', body.classList.contains('nav-open') ? 'true' : 'false');
}

function normalizePathname(pathname){
  const normalized=(pathname || '/').replace(/\/$/, '');
  return normalized || '/';
}

function normalizeLocation(target){
  try{
    const u=new URL(target, window.location.origin);
    return normalizePathname(u.pathname) + u.search + u.hash;
  }catch(_){
    return target || '';
  }
}

function sameLocation(current, target){
  return normalizeLocation(current) === normalizeLocation(target);
}

function stripModalParams(target){
  try{
    const u=new URL(target, window.location.origin);
    u.searchParams.delete('modal');
    return normalizePathname(u.pathname) + u.search + u.hash;
  }catch(_){
    return target;
  }
}

function getDefaultReturnPath(){
  return window.location.pathname + window.location.search;
}

function getModalReturnPath(){
  return modal ? (modal.getAttribute('data-return-path') || getDefaultReturnPath()) : getDefaultReturnPath();
}

function buildModalURL(url, ret){
  try{
    const u=new URL(url, window.location.origin);
    if(!u.searchParams.has('modal')) u.searchParams.set('modal', '1');
    if(ret && !u.searchParams.has('return')) u.searchParams.set('return', ret);
    return u.pathname + u.search + u.hash;
  }catch(_){
    return url;
  }
}

function clearModalCleanup(){
  while(modalCleanupFns.length){
    const fn=modalCleanupFns.pop();
    try{ fn(); }catch(_){}
  }
}

function scheduleHideHoursTooltip(){
  if(!tooltipPortal) return;
  window.clearTimeout(tooltipHideTimer);
  tooltipHideTimer=window.setTimeout(hideHoursTooltip, 120);
}

function cancelHideHoursTooltip(){
  window.clearTimeout(tooltipHideTimer);
}

function ensureTooltipPortal(){
  if(tooltipPortal || !document.body) return;
  tooltipPortal=document.createElement('div');
  tooltipPortal.id='app-hours-tooltip-portal';
  tooltipPortal.className='hours-tooltip-portal';
  tooltipPortal.setAttribute('aria-hidden', 'true');
  tooltipPortal.addEventListener('mouseenter', cancelHideHoursTooltip);
  tooltipPortal.addEventListener('mouseleave', scheduleHideHoursTooltip);
  document.body.appendChild(tooltipPortal);
  body.classList.add('hours-tooltip-portal-ready');
}

function positionHoursTooltip(cell){
  if(!tooltipPortal || !cell || !tooltipPortal.classList.contains('visible')) return;
  const cellRect=cell.getBoundingClientRect();
  const tooltipRect=tooltipPortal.getBoundingClientRect();
  let left=cellRect.left + (cellRect.width / 2) - (tooltipRect.width / 2);
  left=Math.max(12, Math.min(left, window.innerWidth - tooltipRect.width - 12));
  let top=cellRect.top - tooltipRect.height - 12;
  if(top < 96){
    top=cellRect.bottom + 12;
  }
  if(top + tooltipRect.height > window.innerHeight - 12){
    top=Math.max(12, window.innerHeight - tooltipRect.height - 12);
  }
  tooltipPortal.style.left=Math.round(left) + 'px';
  tooltipPortal.style.top=Math.round(top) + 'px';
}

function showHoursTooltip(cell){
  const source=cell ? cell.querySelector('.hours-tooltip') : null;
  if(!source || !source.innerHTML.trim()) return;
  ensureTooltipPortal();
  if(!tooltipPortal) return;
  cancelHideHoursTooltip();
  activeHoursCell=cell;
  tooltipPortal.innerHTML=source.innerHTML;
  tooltipPortal.classList.add('visible');
  tooltipPortal.setAttribute('aria-hidden', 'false');
  positionHoursTooltip(cell);
}

function hideHoursTooltip(){
  cancelHideHoursTooltip();
  activeHoursCell=null;
  if(!tooltipPortal) return;
  tooltipPortal.classList.remove('visible');
  tooltipPortal.setAttribute('aria-hidden', 'true');
  tooltipPortal.innerHTML='';
  tooltipPortal.style.removeProperty('left');
  tooltipPortal.style.removeProperty('top');
}

function bindHoursTooltipListeners(root){
  (root || document).querySelectorAll('.hours-cell').forEach(function(cell){
    if(!cell.querySelector('.hours-tooltip') || cell.hasAttribute('data-hours-tooltip-bound')) return;
    cell.setAttribute('data-hours-tooltip-bound', 'true');
    cell.addEventListener('mouseenter', function(){ showHoursTooltip(cell); });
    cell.addEventListener('mouseleave', function(){
      if(!tooltipPortal || !tooltipPortal.matches(':hover')) scheduleHideHoursTooltip();
    });
    cell.addEventListener('focusin', function(){ showHoursTooltip(cell); });
    cell.addEventListener('focusout', scheduleHideHoursTooltip);
    cell.addEventListener('touchstart', function(){ showHoursTooltip(cell); }, { passive: true });
  });
}

function closeTimesheetMenus(){
  document.querySelectorAll('.timesheet-quick-menu.open').forEach(function(menu){
    menu.classList.remove('open', 'open-up');
    const btn=menu.parentElement ? menu.parentElement.querySelector('[data-timesheet-menu-toggle]') : null;
    if(btn) btn.setAttribute('aria-expanded', 'false');
  });
}

function closeNav(){
  body.classList.remove('nav-open');
  syncNavState();
}

function ensureBrandAssets(){
  const head=document.head || document.getElementsByTagName('head')[0];
  if(!head) return;
  if(!head.querySelector('link[rel="icon"]')){
    const ico=document.createElement('link');
    ico.rel='icon';
    ico.type='image/x-icon';
    ico.href='/static/img/favicon.ico';
    head.appendChild(ico);
  }
  if(!head.querySelector('link[rel="apple-touch-icon"]')){
    const png=document.createElement('link');
    png.rel='apple-touch-icon';
    png.href='/static/img/logo-192.png';
    head.appendChild(png);
  }
  if(!head.querySelector('link[rel="manifest"]')){
    const manifest=document.createElement('link');
    manifest.rel='manifest';
    manifest.href='/manifest.webmanifest';
    head.appendChild(manifest);
  }
  if(!head.querySelector('meta[name="theme-color"]')){
    const theme=document.createElement('meta');
    theme.name='theme-color';
    theme.content='#efe7db';
    head.appendChild(theme);
  }
  if(!head.querySelector('meta[name="apple-mobile-web-app-capable"]')){
    const capable=document.createElement('meta');
    capable.name='apple-mobile-web-app-capable';
    capable.content='yes';
    head.appendChild(capable);
  }
}

function syncThemeMeta(){
  const meta=document.querySelector('meta[name="theme-color"]');
  if(!meta) return;
  const themeColor=getComputedStyle(document.documentElement).getPropertyValue('--theme-color').trim();
  if(themeColor) meta.setAttribute('content', themeColor);
}

function initThemeMeta(){
  syncThemeMeta();
  if(!window.matchMedia) return;
  const media=window.matchMedia('(prefers-color-scheme: dark)');
  const handleChange=function(){
    window.requestAnimationFrame(syncThemeMeta);
  };
  if(media.addEventListener) media.addEventListener('change', handleChange);
  else if(media.addListener) media.addListener(handleChange);
}

function showModalLoading(title){
  if(modalTitle) modalTitle.textContent=title || defaultModalTitle;
  if(!modalContent) return;
  modalContent.innerHTML='<div class="action-modal-loading"><div class="action-modal-loading-bar"></div><p>Загрузка...</p></div>';
  if(modalBody) modalBody.scrollTop=0;
}

function executeModalScripts(scope){
  Array.from(scope.querySelectorAll('script')).forEach(function(oldScript){
    const replacement=document.createElement('script');
    Array.from(oldScript.attributes).forEach(function(attr){
      replacement.setAttribute(attr.name, attr.value);
    });
    replacement.text=oldScript.textContent || '';
    const originalAdd=EventTarget.prototype.addEventListener;
    EventTarget.prototype.addEventListener=function(type, listener, options){
      modalCleanupFns.push(function(){
        try{ this.removeEventListener(type, listener, options); }catch(_){}
      }.bind(this));
      return originalAdd.call(this, type, listener, options);
    };
    try{
      oldScript.replaceWith(replacement);
    } finally {
      EventTarget.prototype.addEventListener=originalAdd;
    }
  });
}

function focusFirstModalField(){
  if(!modalContent) return;
  const field=modalContent.querySelector('input:not([type="hidden"]):not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled])');
  if(field && typeof field.focus === 'function'){
    window.requestAnimationFrame(function(){
      try{ field.focus({ preventScroll: true }); }catch(_){ field.focus(); }
    });
  }
}

function renderModalHTML(html, fallbackTitle, returnPath){
  if(!modalContent) return;
  clearModalCleanup();
  const doc=parser.parseFromString(html, 'text/html');
  const pageTitle=(doc.querySelector('title') && doc.querySelector('title').textContent.trim()) || (doc.querySelector('h1') && doc.querySelector('h1').textContent.trim()) || fallbackTitle || defaultModalTitle;
  if(modalTitle) modalTitle.textContent=pageTitle;
  modalContent.innerHTML=doc.body ? doc.body.innerHTML : html;
  if(modal) modal.setAttribute('data-return-path', returnPath || getDefaultReturnPath());
  if(modalBody) modalBody.scrollTop=0;
  bindHoursTooltipListeners(modalContent);
  executeModalScripts(modalContent);
  focusFirstModalField();
}

function navigateAfterModal(target){
  const cleanTarget=stripModalParams(target || getModalReturnPath());
  closeModal();
  if(sameLocation(window.location.href, cleanTarget)){
    window.location.reload();
    return;
  }
  window.location.assign(cleanTarget);
}

function closeModal(){
  modalRequestSeq += 1;
  hideHoursTooltip();
  clearModalCleanup();
  if(!modal) return;
  modal.classList.remove('visible');
  modal.setAttribute('aria-hidden', 'true');
  modal.removeAttribute('data-return-path');
  body.classList.remove('modal-open');
  if(modalContent) modalContent.innerHTML='';
  if(lastModalTrigger && document.contains(lastModalTrigger)){
    try{ lastModalTrigger.focus({ preventScroll: true }); }catch(_){ lastModalTrigger.focus(); }
  }
  lastModalTrigger=null;
}

function openModal(url, title, ret, trigger){
  if(!modal || !modalContent || !window.fetch){
    window.location.href=url;
    return;
  }
  const effectiveRet=ret || getDefaultReturnPath();
  const requestURL=buildModalURL(url, effectiveRet);
  const seq=++modalRequestSeq;
  lastModalTrigger=trigger || document.activeElement;
  hideHoursTooltip();
  clearModalCleanup();
  modal.classList.add('visible');
  modal.setAttribute('aria-hidden', 'false');
  modal.setAttribute('data-return-path', effectiveRet);
  body.classList.add('modal-open');
  showModalLoading(title);
  fetch(requestURL, {
    credentials: 'same-origin',
    headers: { 'X-Requested-With': 'XMLHttpRequest' }
  }).then(function(response){
    return response.text().then(function(text){
      return { response: response, text: text };
    });
  }).then(function(payload){
    if(seq !== modalRequestSeq) return;
    if(payload.response.redirected){
      navigateAfterModal(payload.response.url);
      return;
    }
    renderModalHTML(payload.text, title, effectiveRet);
  }).catch(function(){
    if(seq !== modalRequestSeq) return;
    window.location.assign(stripModalParams(requestURL));
  });
}

function submitModalForm(form, submitter){
  const method=(form.getAttribute('method') || 'GET').toUpperCase();
  const action=form.getAttribute('action') || window.location.href;
  const returnPath=getModalReturnPath();
  if(method === 'GET'){
    const data=new URLSearchParams(new FormData(form));
    const target=new URL(action, window.location.origin);
    target.search=data.toString();
    openModal(target.pathname + target.search + target.hash, modalTitle ? modalTitle.textContent : defaultModalTitle, returnPath, submitter);
    return;
  }

  const formData=new FormData(form);
  if(submitter && submitter.name && !formData.has(submitter.name)){
    formData.append(submitter.name, submitter.value || '');
  }

  const hasBinary=Array.from(formData.values()).some(function(value){
    return typeof File !== 'undefined' && value instanceof File && value.name;
  });
  const headers={ 'X-Requested-With': 'XMLHttpRequest' };
  let requestBody=formData;
  if(!hasBinary){
    const params=new URLSearchParams();
    formData.forEach(function(value, key){
      if(typeof value === 'string') params.append(key, value);
    });
    requestBody=params;
    headers['Content-Type']='application/x-www-form-urlencoded; charset=UTF-8';
  }

  const seq=++modalRequestSeq;
  if(submitter && typeof submitter.disabled !== 'undefined'){
    submitter.disabled=true;
  }

  fetch(action, {
    method: method,
    body: requestBody,
    credentials: 'same-origin',
    headers: headers
  }).then(function(response){
    return response.text().then(function(text){
      return { response: response, text: text };
    });
  }).then(function(payload){
    if(seq !== modalRequestSeq) return;
    if(payload.response.redirected){
      navigateAfterModal(payload.response.url);
      return;
    }
    renderModalHTML(payload.text, modalTitle ? modalTitle.textContent : defaultModalTitle, returnPath);
  }).catch(function(){
    if(seq !== modalRequestSeq) return;
    window.location.assign(stripModalParams(action));
  }).finally(function(){
    if(submitter && typeof submitter.disabled !== 'undefined'){
      submitter.disabled=false;
    }
  });
}

ensureBrandAssets();
initThemeMeta();
ensureTooltipPortal();

if ('serviceWorker' in navigator) {
  window.addEventListener('load', function(){ navigator.serviceWorker.register('/sw.js').catch(function(){}); });
}

document.addEventListener('click', function(e){
  const trigger=e.target.closest('[data-modal-url]');
  if(trigger){
    e.preventDefault();
    openModal(trigger.getAttribute('data-modal-url'), trigger.getAttribute('data-modal-title'), trigger.getAttribute('data-modal-return'), trigger);
    return;
  }

  const btn=e.target.closest('[data-timesheet-menu-toggle]');
  if(btn){
    e.preventDefault();
    const menu=btn.nextElementSibling;
    if(!menu || !menu.classList.contains('timesheet-quick-menu')) return;
    const isOpen=menu.classList.contains('open');
    closeTimesheetMenus();
    if(isOpen) return;
    menu.classList.add('open');
    menu.classList.remove('open-up');
    btn.setAttribute('aria-expanded', 'true');
    const rect=menu.getBoundingClientRect();
    const needDown=rect.height + 12;
    const triggerRect=btn.getBoundingClientRect();
    const freeDown=window.innerHeight - triggerRect.bottom;
    const freeUp=triggerRect.top;
    if(freeDown < needDown && freeUp > freeDown){
      menu.classList.add('open-up');
    }
    return;
  }

  if(!e.target.closest('.timesheet-quick-menu')) closeTimesheetMenus();
  if(!e.target.closest('.hours-cell') && !e.target.closest('#app-hours-tooltip-portal')) hideHoursTooltip();
});

if(modalContent){
  modalContent.addEventListener('submit', function(e){
    const form=e.target.closest('form');
    if(!form) return;
    if(form.target && form.target !== '_self') return;
    e.preventDefault();
    submitModalForm(form, e.submitter || document.activeElement);
  });

  modalContent.addEventListener('click', function(e){
    const link=e.target.closest('a[href]');
    if(!link || link.hasAttribute('data-modal-url')) return;
    if(link.hasAttribute('download')) return;
    if(link.target && link.target !== '_self') return;
    const rawHref=link.getAttribute('href');
    if(!rawHref || rawHref.charAt(0) === '#' || /^javascript:/i.test(rawHref) || /^mailto:/i.test(rawHref) || /^tel:/i.test(rawHref)) return;
    let target;
    try{
      target=new URL(rawHref, window.location.origin);
    }catch(_){
      return;
    }
    if(target.origin !== window.location.origin) return;
    e.preventDefault();
    navigateAfterModal(target.pathname + target.search + target.hash);
  });
}

if(closeBtn) closeBtn.addEventListener('click', closeModal);
if(modal) modal.addEventListener('click', function(e){
  if(e.target === modal) closeModal();
});
if(burger) burger.addEventListener('click', function(){
  body.classList.toggle('nav-open');
  syncNavState();
});
if(navOverlay) navOverlay.addEventListener('click', closeNav);
document.querySelectorAll('.side-nav-links a').forEach(function(a){
  a.addEventListener('click', closeNav);
});
window.addEventListener('load', function(){
  bindHoursTooltipListeners(document);
});
window.setTimeout(function(){
  bindHoursTooltipListeners(document);
}, 0);
window.addEventListener('resize', function(){
  if(window.innerWidth >= 1024) closeNav();
  if(activeHoursCell) positionHoursTooltip(activeHoursCell);
});
window.addEventListener('scroll', function(){
  if(activeHoursCell) positionHoursTooltip(activeHoursCell);
}, true);
document.addEventListener('keydown', function(e){
  if(e.key === 'Escape'){
    hideHoursTooltip();
    closeTimesheetMenus();
    closeModal();
    closeNav();
  }
});

syncNavState();
})();</script>`

	return fmt.Sprintf(`
<header class="top-nav">
  <div class="container nav-inner">
    <button class="mobile-nav-toggle" type="button" data-mobile-nav-toggle aria-label="Меню" aria-expanded="false">
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <rect x="3.5" y="4.5" width="17" height="15" rx="4"></rect>
        <path d="M8.5 8h8"></path>
        <path d="M8.5 12h8"></path>
        <path d="M8.5 16h5"></path>
        <path d="M6.5 7.5v9"></path>
      </svg>
    </button>
    <div class="top-nav-title-wrap">
      <span class="top-nav-eyebrow">WorkService</span>
      <h1 class="top-nav-title">%s</h1>
    </div>
  </div>
</header>
<div class="side-nav-overlay" data-nav-overlay></div>
<aside class="side-nav" aria-label="Навигация">
  <div class="side-nav-brand"><img src="/static/img/logo.svg" alt="logo"><span>АВАЮССТРОЙ</span></div>
  <a class="side-nav-user" href="/profile"><span class="user-avatar">%s</span><div><strong>%s</strong><small>%s</small></div></a>
  <nav class="side-nav-links">%s</nav>
  <a class="btn btn-secondary side-nav-logout" href="/logout">Выйти</a>
</aside>
<div class="floating-create-wrap"><a class="floating-create-btn" href="/schedule/new" data-modal-url="/schedule/new" data-modal-title="Новое назначение" aria-label="Создать назначение">+</a></div>
<div class="action-modal" id="app-action-modal" aria-hidden="true" role="dialog" aria-modal="true" aria-labelledby="app-action-modal-title">
  <div class="action-modal-sheet">
    <div class="action-modal-header"><strong id="app-action-modal-title">Форма</strong><button type="button" class="action-modal-close" data-modal-close aria-label="Закрыть">&times;</button></div>
    <div class="action-modal-body"><div id="app-action-modal-content" class="action-modal-content"></div></div>
  </div>
</div>%s%s`, pageTitle, userInitial, userName, roleLabel, nav.String(), csrfScript, uiScript)
}

func IsModalRequest(c *gin.Context) bool {
	return c.Query("modal") == "1"
}

func CSRFHiddenInput(c *gin.Context) string {
	csrfToken, _ := c.Get("csrfToken")
	if token, ok := csrfToken.(string); ok && token != "" {
		return `<input type="hidden" name="_csrf_token" value="` + token + `">`
	}
	return ""
}
