package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strconv"

	"project/internal/models"
	"project/internal/storage"

	"github.com/gin-gonic/gin"
)

func canManageInventory(c *gin.Context, object models.Object) bool {
	if isAdmin(c) {
		return true
	}
	userID := c.GetString("userID")
	if userID == "" {
		return false
	}
	worker, err := storage.GetWorkerByUserID(userID)
	if err == nil && worker.IsFired {
		return false
	}
	if object.ResponsibleUserID == userID {
		return true
	}
	if err != nil {
		return false
	}
	entries, err := storage.GetTimesheets()
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.UserMark == "" && slices.Contains(entry.WorkerIDs, worker.ID) && slices.Contains(entry.ObjectIDs, object.ID) {
			return true
		}
	}
	return false
}

func requireInventoryObject(c *gin.Context) (models.Object, bool) {
	object, err := storage.GetObjectByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Объект не найден"})
		return object, false
	}
	if !canManageInventory(c, object) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нет доступа к инвентарю этого объекта"})
		return object, false
	}
	return object, true
}

func ObjectInventory(c *gin.Context) {
	object, ok := requireInventoryObject(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, storage.GetInventory(object.ID))
}

func MutateObjectInventory(c *gin.Context) {
	object, ok := requireInventoryObject(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	if err := c.Request.ParseForm(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать данные позиции"})
		return
	}
	action := c.Param("action")
	if c.Param("itemID") == "" {
		action = "add"
	}
	mutation := storage.InventoryMutation{Action: action, ItemID: c.Param("itemID"), Name: c.PostForm("name"), Unit: c.PostForm("unit"), Note: c.PostForm("note")}
	mutation.Version, _ = strconv.ParseInt(c.PostForm("version"), 10, 64)
	var err error
	switch action {
	case "add", "update", "quantity":
		mutation.Quantity, err = storage.ParseInventoryQuantity(c.PostForm("quantity"))
	case "adjust":
		mutation.Delta, err = strconv.ParseInt(c.PostForm("delta"), 10, 64)
		if err != nil {
			err = storage.ErrInventoryInvalid
		}
	case "delete":
	default:
		err = storage.ErrInventoryInvalid
	}
	if err == nil {
		err = storage.ChangeInventory(object.ID, c.GetString("userID"), c.GetString("userName"), mutation)
	}
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, storage.ErrInventoryNotFound):
			status = http.StatusNotFound
		case errors.Is(err, storage.ErrInventoryConflict), errors.Is(err, storage.ErrInventoryDuplicate):
			status = http.StatusConflict
		case errors.Is(err, storage.ErrInventoryInvalid):
		default:
			log.Printf("save inventory for object %s: %v", object.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить инвентарь. Данные не изменены"})
			return
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, storage.GetInventory(object.ID))
}

var inventorySectionTemplate = template.Must(template.New("inventory").Parse(`
<link rel="stylesheet" href="/static/css/inventory.css?v=1">
<section id="object-inventory" class="inventory-section" aria-labelledby="inventory-title" data-endpoint="/object/{{.ObjectID}}/inventory" data-csrf="{{.CSRF}}">
  <header class="inventory-heading">
    <div class="inventory-heading-title"><h2 id="inventory-title">Инвентарь</h2><span id="inventory-count" class="inventory-count"></span></div>
    <button type="button" class="btn btn-primary" id="inventory-add"><span class="inv-icon inv-plus" aria-hidden="true"></span>Добавить</button>
  </header>
  <div class="inventory-toolbar">
    <label class="inventory-search"><span class="inv-icon inv-search" aria-hidden="true"></span><input id="inventory-search" type="search" aria-label="Поиск инвентаря" placeholder="Найти позицию" autocomplete="off"></label>
    <button type="button" class="inv-button" id="inventory-refresh" aria-label="Обновить инвентарь" title="Обновить инвентарь"><span class="inv-icon inv-refresh" aria-hidden="true"></span></button>
  </div>
  <p id="inventory-status" class="inventory-status" role="status" aria-live="polite"></p>
  <ul id="inventory-items" class="inventory-items" aria-label="Инвентарь объекта"></ul>
  <p id="inventory-empty" class="inventory-empty" hidden></p>
  <details class="inventory-history"><summary>Последние изменения</summary><ol id="inventory-history"></ol></details>
  <noscript>Для работы с инвентарём включите JavaScript.</noscript>
  <dialog id="inventory-editor" class="confirmation-dialog inventory-dialog" aria-labelledby="inventory-editor-title">
    <div class="modal-content">
      <header class="inventory-dialog-heading"><h2 id="inventory-editor-title">Добавить позицию</h2><button type="button" class="inv-button" data-close-dialog aria-label="Закрыть" title="Закрыть"><span class="inv-icon inv-x" aria-hidden="true"></span></button></header>
      <form id="inventory-form" method="post" data-native-submit>
        <button type="button" class="btn btn-secondary" id="inventory-reload-editor" hidden>Загрузить актуальные данные</button>
        <div class="inventory-field"><label for="inventory-name">Наименование</label><input id="inventory-name" name="name" list="inventory-names" maxlength="120" required autocomplete="off"><datalist id="inventory-names"></datalist></div>
        <div class="inventory-form-pair">
          <div class="inventory-field"><label for="inventory-quantity">Количество</label><input id="inventory-quantity" name="quantity" type="text" inputmode="decimal" value="1" required maxlength="11" pattern="[0-9]+([.,][0-9]{1,3})?"></div>
          <div class="inventory-field"><label for="inventory-unit">Единица</label><select id="inventory-unit" name="unit"><option value="pcs">шт.</option><option value="set">компл.</option><option value="pack">упак.</option><option value="m">м</option><option value="m2">м²</option><option value="m3">м³</option><option value="kg">кг</option><option value="l">л</option></select></div>
        </div>
        <p id="inventory-existing" class="inventory-existing" hidden></p>
        <div class="inventory-field"><label for="inventory-note">Примечание <span class="inventory-optional">необязательно</span></label><textarea id="inventory-note" name="note" rows="2" maxlength="500"></textarea></div>
        <div class="inventory-dialog-actions"><button type="button" class="btn btn-secondary" data-close-dialog>Отмена</button><button type="submit" class="btn btn-primary" id="inventory-submit">Добавить</button></div>
      </form>
    </div>
  </dialog>
  <dialog id="inventory-delete" class="confirmation-dialog inventory-dialog" aria-labelledby="inventory-delete-title">
    <div class="modal-content"><h2 id="inventory-delete-title">Удалить позицию?</h2><p id="inventory-delete-name"></p><div class="inventory-dialog-actions"><button type="button" class="btn btn-secondary" data-close-dialog autofocus>Отмена</button><button type="button" class="btn btn-danger" id="inventory-delete-confirm">Удалить</button></div></div>
  </dialog>
  <script type="application/json" id="inventory-data">{{.Data}}</script>
</section>
<script src="/static/js/inventory.js?v=1" defer></script>`))

func renderObjectInventory(c *gin.Context, object models.Object) string {
	if !canManageInventory(c, object) {
		return ""
	}
	data, err := json.Marshal(storage.GetInventory(object.ID))
	if err != nil {
		return ""
	}
	var output bytes.Buffer
	err = inventorySectionTemplate.Execute(&output, struct {
		ObjectID string
		CSRF     string
		Data     template.JS
	}{object.ID, c.GetString("csrfToken"), template.JS(data)}) // json.Marshal escapes HTML, including script delimiters.
	if err != nil {
		log.Printf("render inventory: %v", err)
		return ""
	}
	return output.String()
}
