package api

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"project/internal/construction"
	"project/internal/documentfont"
	"project/internal/models"
	"project/internal/storage"
)

//go:embed templates/construction_documents.html
var constructionHTML string

func renderConstructionDocuments(c *gin.Context, object models.Object) string {
	if !isAdmin(c) {
		return ""
	}
	data := struct{ Endpoint, CSRF string }{"/object/" + object.ID + "/documents", c.GetString("csrfToken")}
	var out bytes.Buffer
	if err := template.Must(template.New("construction").Parse(constructionHTML)).Execute(&out, data); err != nil {
		log.Printf("construction template: %v", err)
		return ""
	}
	return out.String()
}

func constructionObject(c *gin.Context) (models.Object, bool) {
	if !isAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Документы доступны только администраторам"})
		return models.Object{}, false
	}
	object, err := storage.GetObjectByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Объект не найден"})
		return object, false
	}
	c.Header("Cache-Control", "no-store")
	return object, true
}

func ConstructionDocuments(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	docs := storage.GetConstructionDocuments(object.ID)
	rowCounts := map[string]int{}
	for i := range docs {
		rowCounts[docs[i].ID] = len(docs[i].Content.Rows)
		docs[i].Content.Rows = nil
	}
	columns, titles := map[int][]construction.Column{}, map[int]string{}
	for _, s := range construction.JournalSections {
		columns[s.Number] = construction.Columns(s.Number)
		titles[s.Number] = construction.SectionTitle(s.Number)
	}
	automation, err := storage.ConstructionAutomation(object)
	if err != nil {
		c.JSON(500, gin.H{"error": "Не удалось загрузить данные автозаполнения"})
		return
	}
	fontState := gin.H{"name": "Times New Roman", "available": true}
	if _, err := documentfont.Times(); err != nil {
		fontState["available"] = false
		fontState["reason"] = err.Error()
	}
	c.JSON(http.StatusOK, gin.H{"documents": docs, "rowCounts": rowCounts, "object": object, "automation": automation, "weather": documentWeatherStatus(), "font": fontState, "today": time.Now().In(vehicleWaybillLocation).Format("2006-01-02"), "basis": construction.Basis, "schema": gin.H{"requisites": construction.Common, "act": construction.Act, "journal": construction.JournalCover, "sections": construction.JournalSections}, "layouts": gin.H{"act": construction.ActLayout, "journal": construction.CoverLayout, "journalTitle": construction.JournalTitleLayout, "signatories": construction.Signatories, "columns": columns, "titles": titles}})
}

func ConstructionFont(c *gin.Context) {
	data := documentfont.Lookup(c.Param("name"))
	if data == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "font/ttf", data)
}

func ConstructionDocument(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	doc, err := storage.GetConstructionDocument(object.ID, c.Param("documentID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, doc)
}

func SaveConstructionDocument(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
	var request struct {
		ID      string                     `json:"id"`
		Version int64                      `json:"version"`
		Content models.ConstructionContent `json:"content"`
		Reason  string                     `json:"reason"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать документ (максимум 2 МБ)"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Недопустимые данные документа"})
		return
	}
	user, err := storage.GetUserByID(c.GetString("userID"))
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Пользователь не найден"})
		return
	}
	doc, err := storage.SaveConstructionDocument(object.ID, request.ID, request.Version, request.Content, user.ID, user.Name, request.Reason)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, storage.ErrDocumentConflict) {
			status = http.StatusConflict
		}
		if errors.Is(err, storage.ErrDocumentNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, doc)
}

// Assignments are plans, not evidence of completed work. Only descriptions and
// shifts are proposed; measurements, acceptance and signatures stay blank.
func ConstructionSources(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	from, to := c.Query("from"), c.Query("to")
	if !construction.ValidDate(from) || !construction.ValidDate(to) || from > to {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Выберите корректный период"})
		return
	}
	start, _ := time.Parse("2006-01-02", from)
	end, _ := time.Parse("2006-01-02", to)
	if end.Sub(start) > 366*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Выберите период не больше года"})
		return
	}
	section := 4
	if c.Query("section") == "2" {
		section = 2
	}
	rows, err := storage.ConstructionAssignmentRows(object, from, to, section)
	if err != nil {
		c.JSON(500, gin.H{"error": "Не удалось загрузить назначения"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rows": rows})
}

// This proposes draft rows only; opening a journal never writes a revision.
func ConstructionJournalAssignments(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
	var input struct {
		From     string   `json:"from"`
		To       string   `json:"to"`
		Excluded []string `json:"excluded"`
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		c.JSON(400, gin.H{"error": "Не удалось прочитать период назначений"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(400, gin.H{"error": "Ожидался один запрос назначений"})
		return
	}
	today := time.Now().In(vehicleWaybillLocation).Format("2006-01-02")
	openEnd := input.To == ""
	if input.From == "" {
		input.From = "1900-01-01"
	}
	if input.To == "" {
		input.To = today
	}
	if !construction.ValidDate(input.From) || !construction.ValidDate(input.To) || (input.From > input.To && !openEnd) || len(input.Excluded) > 30000 {
		c.JSON(422, gin.H{"error": "Проверьте период и источники записей"})
		return
	}
	excluded := map[string]bool{}
	for _, source := range input.Excluded {
		if !strings.HasPrefix(source, "assignment:") || len(source) > 100 {
			c.JSON(422, gin.H{"error": "Некорректный источник назначения"})
			return
		}
		excluded[source] = true
	}
	if input.To > today {
		input.To = today
	}
	if input.From > input.To {
		c.JSON(200, gin.H{"rows": []models.ConstructionRow{}})
		return
	}
	rows, err := storage.ConstructionJournalAssignmentRows(object, input.From, input.To, excluded)
	if err != nil {
		c.JSON(500, gin.H{"error": "Не удалось загрузить назначения объекта"})
		return
	}
	if len(rows) > 1500 {
		c.JSON(422, gin.H{"error": "Слишком много назначений. Уточните дату начала журнала или добавьте выбранный период через «Из назначений»"})
		return
	}
	for _, row := range rows {
		if len(row.Sources) > 500 {
			c.JSON(422, gin.H{"error": "В одной смене более 500 назначений. Выберите записи вручную"})
			return
		}
	}
	if err := construction.Validate(models.ConstructionContent{Kind: "journal", Fields: map[string]string{}, Rows: rows}); err != nil {
		c.JSON(422, gin.H{"error": "Слишком большой объём данных назначений. Выберите период вручную: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"rows": rows})
}
