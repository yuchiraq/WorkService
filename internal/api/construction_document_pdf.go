package api

import (
	"bytes"
	"fmt"
	"math"
	"mime"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/gin-gonic/gin"
	"project/internal/construction"
	"project/internal/documentfont"
	"project/internal/models"
	"project/internal/storage"
)

func ConstructionDocumentPDF(c *gin.Context) {
	object, ok := constructionObject(c)
	if !ok {
		return
	}
	doc, err := storage.GetConstructionDocument(object.ID, c.Param("documentID"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if value := c.Query("version"); value != "" {
		version, err := strconv.ParseInt(value, 10, 64)
		if err != nil || version < 1 || version > int64(len(doc.History)) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Версия не найдена"})
			return
		}
		rev := doc.History[version-1]
		doc.Content, doc.Version, doc.UpdatedAt = rev.Content, rev.Version, rev.At
	}
	if doc.Content.Kind == "requisites" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Реквизиты не являются печатным документом"})
		return
	}
	if _, err := documentfont.Times(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	data, err := buildConstructionPDF(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сформировать PDF"})
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("%s-v%d.pdf", doc.Content.Number, doc.Version)}))
	c.Data(http.StatusOK, "application/pdf", data)
}

const documentFont = "TimesNewRoman"

func constructionDate(value string) string {
	if d, err := time.Parse("2006-01-02", value); err == nil {
		return d.Format("02.01.2006")
	}
	if d, err := time.Parse("2006-01-02T15:04", value); err == nil {
		return d.Format("02.01.2006 15:04")
	}
	return value
}

func buildConstructionPDF(doc models.ConstructionDocument) ([]byte, error) {
	fonts, fontErr := documentfont.Times()
	if fontErr != nil {
		return nil, fontErr
	}
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(30, 20, 10)
	p.SetAutoPageBreak(true, 20)
	p.AddUTF8FontFromBytes(documentFont, "", fonts.Regular)
	p.AddUTF8FontFromBytes(documentFont, "B", fonts.Bold)
	p.AddUTF8FontFromBytes(documentFont, "I", fonts.Italic)
	p.SetTitle(doc.Content.Number+" - "+doc.Content.Fields["object_name"], true)
	p.SetCreator("WorkService", true)
	stamp, err := time.Parse(time.RFC3339Nano, doc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.SetCreationDate(stamp)
	p.SetModificationDate(stamp)
	p.AliasNbPages("")
	p.SetHeaderFunc(func() {
		if p.PageNo() > 1 {
			p.SetFont(documentFont, "", 13)
			p.SetTextColor(20, 20, 20)
			pageW, _ := p.GetPageSize()
			p.SetXY(0, 10)
			p.CellFormat(pageW, 6, strconv.Itoa(p.PageNo()), "", 0, "C", false, 0, "")
		}
		p.SetXY(30, 20)
	})
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont(documentFont, "", 9)
		p.SetTextColor(75, 75, 75)
		p.MultiCell(0, 4, fmt.Sprintf("Проект для проверки и подписания · %s · Версия %d", doc.Content.Number, doc.Version), "", "C", false)
		if slices.ContainsFunc(doc.Content.Rows, func(row models.ConstructionRow) bool {
			return strings.HasPrefix(row.Fields["weather_source"], "Open-Meteo")
		}) {
			p.SetFont(documentFont, "", 8)
			pageW, _ := p.GetPageSize()
			p.SetX(30)
			p.CellFormat(pageW-40, 3, "Погода: Open-Meteo · CC BY 4.0 · open-meteo.com", "", 0, "C", false, 0, "https://open-meteo.com/")
		}
	})
	if doc.Content.Kind == "journal" {
		constructionTitle(p, doc)
	}
	p.AddPage()
	p.SetTextColor(20, 20, 20)
	p.SetFont(documentFont, "", 13)
	fields := doc.Content.Fields
	need := func(height float64) {
		_, pageH := p.GetPageSize()
		if p.GetY()+height > pageH-20 {
			p.AddPage()
		}
	}
	write := func(text, style string, size, height float64, align string) {
		p.SetFont(documentFont, style, size)
		p.MultiCell(0, height, text, "", align, false)
	}
	layout := construction.ActLayout
	if doc.Content.Kind == "journal" {
		layout = construction.CoverLayout
	}
	for _, block := range layout {
		if block.Optional && strings.TrimSpace(fields[block.Key]) == "" {
			continue
		}
		if block.Key == "deviations" && fields["violations"] != "present" {
			continue
		}
		switch block.Kind {
		case "reference":
			write(block.Text, "", 10, 4, "R")
			p.Ln(4)
		case "title":
			need(20)
			write(block.Text+" № "+doc.Content.Number, "B", 14, 6, "C")
			p.Ln(2)
		case "subtitle":
			write(block.Text, "B", 13, 5.5, "C")
			p.Ln(3)
		case "text":
			need(12)
			text := block.Text
			if strings.Contains(text, "______ пронумерованных") {
				text = strings.Replace(text, "______", "{nb}", 1)
			}
			write(text, "", 13, 5.5, "L")
			p.Ln(2)
		case "signature-line":
			need(10)
			write("Подпись ____________________", "", 13, 5.5, "L")
			p.Ln(2)
		case "signatures":
			constructionSignatures(p, fields)
		case "date":
			need(14)
			value := constructionDate(fields[block.Key])
			if block.Key == "act_date" && fields[block.Key] != "" {
				if d, e := time.Parse("2006-01-02", fields[block.Key]); e == nil {
					months := []string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}
					value = fmt.Sprintf("«%02d» %s %d г.", d.Day(), months[d.Month()-1], d.Year())
				}
			}
			if value == "" {
				value = "____________________"
			}
			write(block.Text+": "+value, "", 13, 5.5, "L")
			p.Ln(3)
		case "violations":
			need(22)
			value := "отсутствуют / допущены (результат не выбран)"
			if fields[block.Key] == "none" {
				value = "отсутствуют"
			}
			if fields[block.Key] == "present" {
				value = "допущены"
			}
			write(block.Text+" "+value+" "+block.Hint, "", 13, 5.5, "L")
			p.Ln(3)
		default:
			need(25)
			if block.Text != "" {
				write(block.Text, "", 13, 5.5, "L")
			}
			value := fields[block.Key]
			if value == "" {
				y := p.GetY()
				p.SetDrawColor(90, 90, 90)
				p.Line(30, y+5, 200, y+5)
				p.Ln(6)
			} else {
				write(value, "", 13, 5.5, "L")
			}
			if block.Hint != "" {
				p.SetTextColor(65, 65, 65)
				write("("+block.Hint+")", "", 10, 4, "C")
				p.SetTextColor(20, 20, 20)
			}
			p.Ln(3)
		}
	}
	if doc.Content.Kind == "journal" {
		for _, spec := range construction.JournalSections {
			columns := construction.Columns(spec.Number)
			newPage := func() {
				p.AddPageFormat("L", fpdf.SizeType{Wd: 210, Ht: 297})
				p.SetTextColor(20, 20, 20)
				write(fmt.Sprintf("Раздел %d\n%s", spec.Number, construction.SectionTitle(spec.Number)), "B", 13, 5.5, "C")
				name := []rune(fields["object_name"])
				if len(name) > 120 {
					name = append(name[:120], '.', '.', '.')
				}
				write("Журнал № "+doc.Content.Number+" · "+string(name), "", 10, 4, "L")
				p.Ln(3)
				head := make([]string, len(columns))
				for i, col := range columns {
					head[i] = col.Label
				}
				p.SetFont(documentFont, "", 11)
				constructionGrid(p, columns, head, nil, true)
			}
			newPage()
			count := 0
			for _, row := range doc.Content.Rows {
				if row.Section != spec.Number {
					continue
				}
				count++
				values := make([]string, len(columns))
				for i, col := range columns {
					var parts []string
					for _, key := range col.Keys {
						if row.Fields[key] != "" {
							value := constructionDate(row.Fields[key])
							if key == "crew" {
								value = "Бригада (по назначениям):\n" + value
							}
							parts = append(parts, value)
						}
					}
					values[i] = strings.Join(parts, "\n")
					if col.Keys[0] == "_number" {
						values[i] = strconv.Itoa(count)
					}
					if col.Keys[0] == "_signature" {
						values[i] = "\n________"
					}
				}
				if !row.Reviewed {
					values[0] = "ЧЕРНОВИК\n" + values[0]
				}
				constructionGrid(p, columns, values, newPage, false)
			}
			if count == 0 {
				for i := 0; i < 3; i++ {
					empty := make([]string, len(columns))
					empty[0] = "\n\n"
					constructionGrid(p, columns, empty, newPage, false)
				}
			}
		}
	}
	var output bytes.Buffer
	if err := p.Output(&output); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func constructionTitle(p *fpdf.Fpdf, doc models.ConstructionDocument) {
	p.AddPage()
	p.SetTextColor(20, 20, 20)
	for _, block := range construction.JournalTitleLayout {
		switch block.Kind {
		case "reference":
			p.SetFont(documentFont, "", 11)
			p.MultiCell(0, 5, block.Text, "", "R", false)
		case "title":
			p.SetY(103)
			p.SetFont(documentFont, "B", 26)
			p.MultiCell(0, 12, block.Text, "", "C", false)
		case "subtitle":
			p.SetFont(documentFont, "B", 22)
			p.MultiCell(0, 10, block.Text, "", "C", false)
			p.Ln(8)
			p.SetFont(documentFont, "", 14)
			p.MultiCell(0, 6, "№ "+doc.Content.Number, "", "C", false)
		case "cover-summary":
			p.SetY(180)
			p.SetFont(documentFont, "", 12)
			for _, key := range []string{"object_name", "address", "contractor"} {
				value := strings.TrimSpace(doc.Content.Fields[key])
				// The title identifies the volume; full requisites follow in section 1.
				if text := []rune(value); len(text) > 260 {
					value = string(text[:260]) + "…"
				}
				if value != "" {
					p.MultiCell(0, 5, value, "", "C", false)
					p.Ln(3)
				}
			}
		}
	}
}

func constructionSignatures(p *fpdf.Fpdf, fields map[string]string) {
	_, pageH := p.GetPageSize()
	if p.GetY() > pageH-45 {
		p.AddPage()
	}
	p.Ln(3)
	p.SetFont(documentFont, "B", 13)
	p.MultiCell(0, 6, "Подписи представителей", "", "L", false)
	p.Ln(2)
	widths := []float64{61, 36, 25, 48}
	head := []string{"Представитель", "Дата и время", "Подпись", "Фамилия, инициалы"}
	drawHead := func() {
		x, y := 30.0, p.GetY()
		p.SetFont(documentFont, "", 10)
		for i, label := range head {
			p.SetXY(x, y)
			p.MultiCell(widths[i], 4, label, "", "L", false)
			x += widths[i]
		}
		p.SetXY(30, y+10)
	}
	drawHead()
	for _, person := range construction.Signatories {
		values := []string{person.Role, constructionDate(fields[person.At]), "________", fields[person.Name]}
		if values[1] == "" {
			values[1] = "____________"
		}
		if values[3] == "" {
			values[3] = "________________"
		}
		p.SetFont(documentFont, "", 13)
		height := 12.0
		for i, value := range values {
			height = math.Max(height, float64(len(p.SplitText(value, widths[i]-3)))*5.5+5)
		}
		if p.GetY()+height > pageH-20 {
			p.AddPage()
			drawHead()
			p.SetFont(documentFont, "", 13)
		}
		x, y := 30.0, p.GetY()
		for i, value := range values {
			p.SetXY(x, y)
			p.MultiCell(widths[i]-3, 5.5, value, "", "L", false)
			x += widths[i]
		}
		p.SetXY(30, y+height)
	}
	p.Ln(3)
}

// Tall rows continue on a new page, with repeated headings and a visible row
// identifier. Text never shrinks to force a record onto one sheet.
func constructionGrid(p *fpdf.Fpdf, columns []construction.Column, values []string, nextPage func(), header bool) {
	lines := make([][]string, len(columns))
	maximum := 1
	lineHeight := 5.0
	for i, col := range columns {
		value := values[i]
		if !header && slices.Contains(col.Keys, "_signature") && col.Keys[0] != "_signature" {
			value += "\n\nПодписи: __________"
		}
		lines[i] = p.SplitText(value, col.Width-3)
		if len(lines[i]) == 0 {
			lines[i] = []string{""}
		}
		maximum = max(maximum, len(lines[i]))
	}
	for offset := 0; offset < maximum; {
		_, pageH := p.GetPageSize()
		available := int(math.Floor((pageH - 20 - p.GetY() - 3) / lineHeight))
		if available < 1 && nextPage != nil {
			nextPage()
			continue
		}
		available = max(available, 1)
		count := min(maximum-offset, available)
		height := float64(count)*lineHeight + 3
		x, y := 30.0, p.GetY()
		p.SetDrawColor(70, 70, 70)
		p.SetLineWidth(.2)
		for i, col := range columns {
			p.Rect(x, y, col.Width, height, "D")
			cell := lines[i]
			start := offset
			if !header && i == 0 && offset >= len(cell) {
				cell = p.SplitText("Продолжение\n"+values[0], col.Width-3)
				start = 0
			}
			for j := 0; j < count && start+j < len(cell); j++ {
				p.SetXY(x+1.5, y+1.5+float64(j)*lineHeight)
				p.CellFormat(col.Width-3, lineHeight, cell[start+j], "", 0, "L", false, 0, "")
			}
			x += col.Width
		}
		p.SetXY(30, y+height)
		offset += count
		if offset < maximum && nextPage != nil {
			nextPage()
		}
	}
}
