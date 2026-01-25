package api

import (
	"fmt"
)

// RenderSidebar generates the HTML for the sidebar, marking the specified page as active.
func RenderSidebar(activePage string) string {
	items := ""

	dashboardClass := ""
	if activePage == "dashboard" {
		dashboardClass = "active"
	}

	workersClass := ""
	if activePage == "workers" {
		workersClass = "active"
	}

	items += fmt.Sprintf(`<li class="%s"><a href="/dashboard">Главная</a></li>`, dashboardClass)
	items += fmt.Sprintf(`<li class="%s"><a href="/workers">Работники</a></li>`, workersClass)

	return fmt.Sprintf(`
<aside class="sidebar">
	<div class="sidebar-header"><h2>Управление</h2></div>
	<nav><ul>%s</ul></nav>
	<div class="sidebar-footer"><a href="/logout">Выход</a></div>
</aside>`, items)
}
