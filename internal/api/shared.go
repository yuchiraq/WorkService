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
	}
	if userStatus == "admin" {
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
const iframe=document.getElementById('app-action-modal-iframe');
const closeBtn=document.querySelector('[data-modal-close]');
const modalSheet=document.querySelector('.action-modal-sheet');
const modalTitle=document.getElementById('app-action-modal-title');
const burger=document.querySelector('[data-mobile-nav-toggle]');
const navOverlay=document.querySelector('[data-nav-overlay]');
const themeButtons=document.querySelectorAll('[data-theme-option]');
const themeStorageKey='workservice-theme';

function closeModal(){
  if(!modal) return;
  modal.classList.remove('visible');
  modal.setAttribute('aria-hidden','true');
  body.classList.remove('modal-open');
  if(iframe){ iframe.removeAttribute('src'); iframe.removeAttribute('data-return-path'); iframe.style.removeProperty('height'); }
}
function closeNav(){ body.classList.remove('nav-open'); }
function openModal(url,t,ret){
  if(!modal||!iframe){ window.location.href=url; return; }
  const effectiveRet = ret || (window.location.pathname + window.location.search);
  try{ const u=new URL(url,window.location.origin); if(!u.searchParams.has('modal')) u.searchParams.set('modal','1'); if(effectiveRet && !u.searchParams.has('return')) u.searchParams.set('return', effectiveRet); url=u.pathname+u.search; }catch(_){ }
  modal.classList.add('visible');
  modal.setAttribute('aria-hidden','false');
  body.classList.add('modal-open');
  if(modalTitle) modalTitle.textContent=t||'Форма';
  iframe.setAttribute('data-return-path', effectiveRet);
  iframe.src=url;
}

function ensureBrandAssets(){
  const head=document.head||document.getElementsByTagName('head')[0];
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

function storedTheme(){
  try{
    const value=window.localStorage.getItem(themeStorageKey);
    return value==='dark'||value==='light' ? value : '';
  }catch(_){
    return '';
  }
}
function systemTheme(){
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
function syncThemeMeta(){
  const meta=document.querySelector('meta[name="theme-color"]');
  if(!meta) return;
  const themeColor=getComputedStyle(document.documentElement).getPropertyValue('--theme-color').trim();
  if(themeColor) meta.setAttribute('content', themeColor);
}
function syncThemeButtons(theme){
  themeButtons.forEach(function(btn){
    const active=btn.getAttribute('data-theme-option')===theme;
    btn.setAttribute('aria-pressed', active ? 'true' : 'false');
    btn.classList.toggle('is-active', active);
  });
}
function applyTheme(theme, persist){
  const resolved=theme==='dark' ? 'dark' : 'light';
  document.documentElement.setAttribute('data-theme', resolved);
  document.documentElement.style.colorScheme=resolved;
  if(persist){
    try{ window.localStorage.setItem(themeStorageKey, resolved); }catch(_){}
  }
  syncThemeButtons(resolved);
  syncThemeMeta();
}
function initTheme(){
  applyTheme(storedTheme() || systemTheme(), false);
  themeButtons.forEach(function(btn){
    btn.addEventListener('click', function(){
      applyTheme(btn.getAttribute('data-theme-option'), true);
    });
  });
  if(!window.matchMedia) return;
  const media=window.matchMedia('(prefers-color-scheme: dark)');
  const handleChange=function(){
    if(storedTheme()) return;
    applyTheme(systemTheme(), false);
  };
  if(media.addEventListener) media.addEventListener('change', handleChange);
  else if(media.addListener) media.addListener(handleChange);
}

function sameTarget(current, target){
  try{
    const c=new URL(current, window.location.origin);
    const t=new URL(target, window.location.origin);
    const cp=c.pathname.replace(/\/$/, '');
    const tp=t.pathname.replace(/\/$/, '');
    return cp===tp;
  }catch(_){ return false; }
}

ensureBrandAssets();
initTheme();
if ('serviceWorker' in navigator) {
  window.addEventListener('load', function(){ navigator.serviceWorker.register('/sw.js').catch(function(){}); });
}
document.addEventListener('click',function(e){
  const t=e.target.closest('[data-modal-url]');
  if(!t) return;
  e.preventDefault();
  openModal(t.getAttribute('data-modal-url'), t.getAttribute('data-modal-title'), t.getAttribute('data-modal-return'));
});
function closeTimesheetMenus(){
  document.querySelectorAll('.timesheet-quick-menu.open').forEach(function(menu){
    menu.classList.remove('open','open-up');
    const btn = menu.parentElement ? menu.parentElement.querySelector('[data-timesheet-menu-toggle]') : null;
    if(btn) btn.setAttribute('aria-expanded','false');
  });
}
document.addEventListener('click', function(e){
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
    btn.setAttribute('aria-expanded','true');
    const rect=menu.getBoundingClientRect();
    const needDown=rect.height + 12;
    const freeDown=window.innerHeight - btn.getBoundingClientRect().bottom;
    const freeUp=btn.getBoundingClientRect().top;
    if(freeDown < needDown && freeUp > freeDown){ menu.classList.add('open-up'); }
    return;
  }
  if(!e.target.closest('.timesheet-quick-menu')) closeTimesheetMenus();
});
if(closeBtn) closeBtn.addEventListener('click',closeModal);
if(modal) modal.addEventListener('click',function(e){ if(e.target===modal) closeModal();});
if(burger) burger.addEventListener('click', function(){ body.classList.toggle('nav-open'); });
if(navOverlay) navOverlay.addEventListener('click', closeNav);
document.querySelectorAll('.side-nav-links a').forEach(function(a){ a.addEventListener('click', closeNav); });
window.addEventListener('resize', function(){ if(window.innerWidth >= 1024) closeNav(); });
document.addEventListener('keydown',function(e){ if(e.key==='Escape'){ closeModal(); closeNav(); }});
if(iframe){
  iframe.addEventListener('load',function(){
    const ret=iframe.getAttribute('data-return-path');
    let href='';
    try{
      href=iframe.contentWindow.location.href;
      if(iframe.contentWindow && iframe.contentWindow.document){
        const d=iframe.contentWindow.document;
        const h=Math.max(d.body ? d.body.scrollHeight : 0, d.documentElement ? d.documentElement.scrollHeight : 0);
        if(h>0){
          const maxH = window.innerHeight - 140;
          iframe.style.height = Math.min(Math.max(h + 14, 320), maxH) + 'px';
          if(modalSheet) modalSheet.style.height = 'auto';
        }
      }
    }catch(_){ return; }
    if(ret && sameTarget(href, ret)){ closeModal(); window.location.assign(ret); }
  });
}
})();</script>`

	return fmt.Sprintf(`
<header class="top-nav">
  <div class="container nav-inner">
    <button class="mobile-nav-toggle" type="button" data-mobile-nav-toggle aria-label="Меню">
      <span></span><span></span><span></span>
    </button>
    <div class="top-nav-title-wrap">
      <span class="top-nav-eyebrow">WorkService</span>
      <h1 class="top-nav-title">%s</h1>
    </div>
    <div class="top-nav-actions">
      <div class="theme-toggle" role="group" aria-label="Theme">
        <button class="theme-toggle-option" type="button" data-theme-option="light" aria-pressed="false">Day</button>
        <button class="theme-toggle-option" type="button" data-theme-option="dark" aria-pressed="false">Night</button>
      </div>
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
<div class="action-modal" id="app-action-modal" aria-hidden="true">
  <div class="action-modal-sheet">
    <div class="action-modal-header"><strong id="app-action-modal-title">Форма</strong><button type="button" class="action-modal-close" data-modal-close aria-label="Закрыть">✕</button></div>
    <iframe id="app-action-modal-iframe" title="Форма"></iframe>
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
