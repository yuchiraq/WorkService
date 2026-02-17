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
	Icon   string
	Label  string
}

// RenderSidebar generates the HTML for the sidebar, marking the specified page as active.
func RenderSidebar(c *gin.Context, activePage string) string {
	userNameValue, _ := c.Get("userName")
	userStatusValue, _ := c.Get("userStatus")
	userName := userNameValue.(string)
	userStatus := userStatusValue.(string)

	navItems := []navItem{
		{PageID: "dashboard", Path: "/dashboard", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"></path><polyline points="9 22 9 12 15 12 15 22"></polyline></svg>`, Label: "Панель управления"},
		{PageID: "workers", Path: "/workers", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>`, Label: "Работники"},
		{PageID: "objects", Path: "/objects", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"></rect><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"></path></svg>`, Label: "Объекты"},
		{PageID: "schedule", Path: "/schedule", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2"></rect><line x1="8" y1="2" x2="8" y2="6"></line><line x1="16" y1="2" x2="16" y2="6"></line><line x1="3" y1="10" x2="21" y2="10"></line></svg>`, Label: "Расписание"},
		{PageID: "timesheets", Path: "/timesheets", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 11h6"></path><path d="M9 15h6"></path><path d="M9 7h6"></path><rect x="3" y="4" width="18" height="16" rx="2"></rect></svg>`, Label: "Табель"},
	}

	if userStatus == "admin" {
		navItems = append(navItems, navItem{PageID: "users", Path: "/users", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 11v6"></path><path d="M20 14h6"></path></svg>`, Label: "Пользователи"})
		navItems = append(navItems, navItem{PageID: "settings", Path: "/settings", Icon: `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h.01a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>`, Label: "Настройки"})
	}

	var itemsBuilder strings.Builder
	for _, item := range navItems {
		class := ""
		if activePage == item.PageID {
			class = "active"
		}
		itemsBuilder.WriteString(fmt.Sprintf(`<li class="%s"><a href="%s">%s<span>%s</span></a></li>`, class, item.Path, item.Icon, item.Label))
	}

	userInitial := ""
	if utf8.RuneCountInString(userName) > 0 {
		firstRune, _ := utf8.DecodeRuneInString(userName)
		userInitial = strings.ToUpper(string(firstRune))
	}

	roleLabel := "Пользователь"

	csrfToken, _ := c.Get("csrfToken")
	csrfScript := ""
	if token, ok := csrfToken.(string); ok && token != "" {
		csrfScript = fmt.Sprintf(`<script>(function(){const t=%q;document.querySelectorAll('form[method="POST"], form[method="post"]').forEach(function(f){if(f.querySelector('input[name="_csrf_token"]')) return; var i=document.createElement('input'); i.type='hidden'; i.name='_csrf_token'; i.value=t; f.appendChild(i);});})();</script>`, token)
	}
	if userStatus == "admin" {
		roleLabel = "Администратор"
	}

	header := `
	<div class="sidebar-header">
		<div class="logo">
		<img src="/static/img/logo.svg" alt="logo"/>
		</div>
		<h2 class="company-name">АВАЮССТРОЙ</h2>
	</div>`

	footer := fmt.Sprintf(`
	<div class="sidebar-footer">
		<a href="/profile" class="user-profile-link">
			<div class="user-profile">
				<div class="user-avatar"><span>%s</span></div>
				<div class="user-info">
					<span class="user-name">%s</span>
					<span class="user-role">%s</span>
				</div>
			</div>
		</a>
		<a href="/logout" class="logout-link">
			<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path><polyline points="16 17 21 12 16 7"></polyline><line x1="21" y1="12" x2="9" y2="12"></line></svg>
			<span>Выйти</span>
		</a>
	</div>`, userInitial, userName, roleLabel)

	return fmt.Sprintf(`
	<aside class="sidebar">
		<div class="sidebar-content">
			%s
			<nav><ul>%s</ul></nav>
		</div>
		%s
	</aside>%s`, header, itemsBuilder.String(), footer, csrfScript)
}
