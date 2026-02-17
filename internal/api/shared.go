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

// RenderSidebar keeps historical name, but renders top glass navigation.
func RenderSidebar(c *gin.Context, activePage string) string {
	userNameValue, _ := c.Get("userName")
	userStatusValue, _ := c.Get("userStatus")
	userName := userNameValue.(string)
	userStatus := userStatusValue.(string)

	navItems := []navItem{
		{PageID: "dashboard", Path: "/dashboard", Label: "Панель"},
		{PageID: "workers", Path: "/workers", Label: "Работники"},
		{PageID: "objects", Path: "/objects", Label: "Объекты"},
		{PageID: "schedule", Path: "/schedule", Label: "Расписание"},
		{PageID: "timesheets", Path: "/timesheets", Label: "Табель"},
	}
	if userStatus == "admin" {
		navItems = append(navItems,
			navItem{PageID: "users", Path: "/users", Label: "Пользователи"},
			navItem{PageID: "settings", Path: "/settings", Label: "Настройки"},
		)
	}

	var nav strings.Builder
	for _, item := range navItems {
		class := "nav-link"
		if item.PageID == activePage {
			class += " active"
		}
		nav.WriteString(fmt.Sprintf(`<a class="%s" href="%s">%s</a>`, class, item.Path, item.Label))
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
const title=document.getElementById('app-action-modal-title');
const closeBtn=document.querySelector('[data-modal-close]');
const burger=document.querySelector('[data-mobile-nav-toggle]');

function closeModal(){
  if(!modal) return;
  modal.classList.remove('visible');
  body.classList.remove('modal-open');
  if(iframe){ iframe.removeAttribute('src'); iframe.removeAttribute('data-return-path'); }
}
function closeNav(){ body.classList.remove('nav-open'); }
function openModal(url,t,ret){
  if(!modal||!iframe){ window.location.href=url; return; }
  try{ const u=new URL(url,window.location.origin); if(!u.searchParams.has('modal')) u.searchParams.set('modal','1'); url=u.pathname+u.search; }catch(_){ }
  modal.classList.add('visible');
  body.classList.add('modal-open');
  title.textContent=t||'Форма';
  iframe.setAttribute('data-return-path', ret||window.location.pathname);
  iframe.src=url;
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

document.addEventListener('click',function(e){
  const t=e.target.closest('[data-modal-url]');
  if(!t) return;
  e.preventDefault();
  openModal(t.getAttribute('data-modal-url'), t.getAttribute('data-modal-title'), t.getAttribute('data-modal-return'));
});
if(closeBtn) closeBtn.addEventListener('click',closeModal);
if(modal) modal.addEventListener('click',function(e){ if(e.target===modal) closeModal();});
if(burger) burger.addEventListener('click', function(){ body.classList.toggle('nav-open'); });
document.querySelectorAll('.nav-links a').forEach(function(a){ a.addEventListener('click', closeNav); });
document.addEventListener('keydown',function(e){ if(e.key==='Escape'){ closeModal(); closeNav(); }});
if(iframe){
  iframe.addEventListener('load',function(){
    const ret=iframe.getAttribute('data-return-path');
    let href='';
    try{ href=iframe.contentWindow.location.href; }catch(_){ return; }
    if(ret && sameTarget(href, ret)){ closeModal(); window.location.href=ret; }
  });
}
})();</script>`

	return fmt.Sprintf(`
<header class="top-nav">
  <div class="container nav-inner">
    <button class="mobile-nav-toggle" type="button" data-mobile-nav-toggle aria-label="Меню">☰</button>
    <div class="brand"><img src="/static/img/logo.svg" alt="logo"><span>АВАЮССТРОЙ</span></div>
    <nav class="nav-links">%s</nav>
    <div class="nav-user"><span class="user-avatar">%s</span><div><strong>%s</strong><small>%s</small></div><a class="nav-link" href="/logout">Выйти</a></div>
  </div>
</header>
<div class="action-modal" id="app-action-modal" aria-hidden="true">
  <div class="action-modal-sheet">
    <div class="action-modal-header"><h3 id="app-action-modal-title">Форма</h3><button type="button" class="action-modal-close" data-modal-close aria-label="Закрыть">✕</button></div>
    <iframe id="app-action-modal-iframe" title="Форма"></iframe>
  </div>
</div>%s%s`, nav.String(), userInitial, userName, roleLabel, csrfScript, uiScript)
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
