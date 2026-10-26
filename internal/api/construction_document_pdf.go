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
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"project/internal/construction"
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
	data, err := buildConstructionPDF(doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сформировать PDF"})
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("%s-v%d.pdf", doc.Content.Number, doc.Version)}))
	c.Data(http.StatusOK, "application/pdf", data)
}

func buildConstructionPDF(doc models.ConstructionDocument) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(12, 14, 12)
	p.SetAutoPageBreak(true, 17)
	p.AddUTF8FontFromBytes("Go", "", goregular.TTF)
	p.AddUTF8FontFromBytes("Go", "B", gobold.TTF)
	p.SetTitle(doc.Content.Number+" - "+doc.Content.Fields["object_name"], true)
	p.SetCreator("WorkService", true)
	stamp, err := time.Parse(time.RFC3339Nano, doc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.SetCreationDate(stamp)
	p.SetModificationDate(stamp)
	p.AliasNbPages("")
	p.SetFooterFunc(func() {
		p.SetY(-13)
		p.SetFont("Go", "", 8)
		p.SetTextColor(85, 85, 85)
		p.CellFormat(0, 4, fmt.Sprintf("Проект для проверки и подписания · Версия %d · Страница %d/{nb}", doc.Version, p.PageNo()), "", 1, "C", false, 0, "")
		p.CellFormat(0, 4, "СН 1.03.04-2020, изменение № 1. Подписи оформляются уполномоченными лицами.", "", 0, "C", false, 0, "")
	})
	p.AddPage()
	p.SetTextColor(20, 20, 20)
	paragraph := func(label, value string) {
		if p.GetY() > 251 {
			p.AddPage()
		}
		p.SetFont("Go", "B", 10)
		p.MultiCell(0, 5, label, "", "L", false)
		p.SetFont("Go", "", 10)
		if strings.TrimSpace(value) == "" {
			value = "________________________________________________________________"
		}
		p.MultiCell(0, 5, value, "", "L", false)
		p.Ln(2)
	}
	title := func(value string) { p.SetFont("Go", "B", 14); p.MultiCell(0, 7, value, "", "C", false); p.Ln(3) }
	f := doc.Content.Fields
	formatDate := func(v string) string {
		if d, e := time.Parse("2006-01-02", v); e == nil {
			return d.Format("02.01.2006")
		}
		return v
	}
	if doc.Content.Kind == "act" {
		title("АКТ № " + doc.Content.Number + "\nосвидетельствования скрытых работ")
		paragraph("Наименование работ", f["work_name"])
		paragraph("Выполненных в (наименование и местонахождение объекта)", f["object_name"]+"\n"+f["address"])
		paragraph("Дата освидетельствования", formatDate(f["act_date"]))
		p.SetFont("Go", "B", 11)
		p.MultiCell(0, 6, "Комиссия в составе:", "", "L", false)
		for _, person := range []struct{ key, label string }{{"contractor_rep", "Представитель подрядчика"}, {"subcontractor_rep", "Представитель субподрядной организации (при её участии)"}, {"technical_rep", "Представитель технического надзора"}, {"author_rep", "Представитель авторского надзора"}} {
			paragraph(person.label+" (должность, фамилия, инициалы)", f[person.key])
		}
		paragraph("Произвела осмотр работ, выполненных (строительно-монтажная организация)", f["performer"])
		p.SetFont("Go", "", 10)
		p.MultiCell(0, 5, "И составила настоящий акт о нижеследующем:", "", "L", false)
		p.Ln(2)
		for _, field := range construction.Act[2:5] {
			paragraph(field.Label, f[field.Key])
		}
		violations := "Не указано (требуется проверка)"
		if f["violations"] == "none" {
			violations = "Отсутствуют"
		} else if f["violations"] == "present" {
			violations = "Допущены\n" + f["deviations"]
		}
		paragraph("4. Нарушения требований ТНПА и (или) проектной документации", violations)
		paragraph("5. Даты начала и окончания работ", "Начало: "+formatDate(f["start_date"])+"\nОкончание: "+formatDate(f["end_date"]))
		if f["attachments"] != "" {
			paragraph("Дополнительные сведения и приложения", f["attachments"])
		}
		if p.GetY() > 178 {
			p.AddPage()
		}
		p.SetFont("Go", "B", 11)
		p.MultiCell(0, 6, "Подписи представителей", "", "L", false)
		p.Ln(3)
		for _, role := range []string{"Подрядчика", "Субподрядной организации (при её участии)", "Технического надзора", "Авторского надзора"} {
			p.SetFont("Go", "", 10)
			p.MultiCell(0, 5, role, "", "L", false)
			p.MultiCell(0, 7, "__________________    ______________    __________________________", "", "L", false)
			p.SetFont("Go", "", 8)
			p.MultiCell(0, 4, "дата и время                         подпись                       фамилия, инициалы", "", "L", false)
			p.Ln(3)
		}
	} else {
		title("ЖУРНАЛ ПРОИЗВОДСТВА РАБОТ\n№ " + doc.Content.Number)
		p.SetFont("Go", "B", 11)
		p.MultiCell(0, 7, "Раздел 1. Общие сведения", "", "L", false)
		p.Ln(2)
		for _, field := range construction.JournalCover {
			value := f[field.Key]
			if field.Type == "date" {
				value = formatDate(value)
			}
			paragraph(field.Label, value)
			if field.Key == "responsible" || field.Key == "designer" || field.Key == "customer" || field.Key == "technical_rep" || field.Key == "issuer" {
				p.SetFont("Go", "", 9)
				p.MultiCell(0, 6, "Подпись ____________________", "", "L", false)
				p.Ln(2)
			}
		}
		paragraph("Оформление бумажного журнала", "В настоящем журнале ______ пронумерованных и прошнурованных страниц.\nПодпись руководителя ____________________\nПечать организации (при её применении) ____________________")
		for _, spec := range construction.JournalSections {
			columns := constructionColumns(spec.Number)
			newPage := func() {
				p.AddPageFormat("L", fpdf.SizeType{Wd: 210, Ht: 297})
				p.SetTextColor(20, 20, 20)
				p.SetFont("Go", "B", 11)
				titles := map[int]string{
					2: "Список инженерно-технического персонала, занятого на строительстве объекта",
					3: "Перечень актов промежуточной приемки ответственных конструкций и освидетельствования скрытых работ",
					4: "Сведения о производстве строительно-монтажных работ с учетом результатов операционного контроля их качества",
					5: "Перечень специальных журналов работ",
					6: "Замечания лиц, контролирующих производство и безопасность работ",
					7: "Сведения о мероприятиях технического (технологического, поверочного) характера в рамках контроля (надзора)",
				}
				p.MultiCell(0, 6, fmt.Sprintf("Раздел %d. %s", spec.Number, titles[spec.Number]), "", "L", false)
				p.SetFont("Go", "", 8)
				objectName := []rune(f["object_name"])
				if len(objectName) > 130 {
					objectName = append(objectName[:130], []rune("...")...)
				}
				p.MultiCell(0, 4, "Журнал № "+doc.Content.Number+" · "+string(objectName), "", "L", false)
				p.Ln(3)
				p.SetFont("Go", "B", 8.5)
				head := make([]string, len(columns))
				for i, col := range columns {
					head[i] = col.label
				}
				constructionGrid(p, columns, head, nil, true)
				p.SetFont("Go", "", 9)
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
					parts := []string{}
					for _, key := range col.keys {
						if row.Fields[key] != "" {
							parts = append(parts, formatDate(row.Fields[key]))
						}
					}
					values[i] = strings.Join(parts, "\n")
					if col.keys[0] == "_number" {
						values[i] = strconv.Itoa(count)
					}
					if col.keys[0] == "_signature" {
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

type constructionColumn struct {
	label string
	keys  []string
	width float64
}

func constructionColumns(section int) []constructionColumn {
	col := func(label string, width float64, keys ...string) constructionColumn {
		return constructionColumn{label, keys, width}
	}
	switch section {
	case 2:
		return []constructionColumn{col("ФИО, должность, участок работ", 100, "person"), col("Дата начала работ", 38, "start"), col("Разрешение / аттестация", 97, "qualification"), col("Дата окончания работ", 38, "end")}
	case 3:
		return []constructionColumn{col("Наименование, номер акта и расположение конструкций / работ", 150, "act"), col("Дата подписания, фамилии, инициалы и должности подписавших", 123, "signed_date", "signers")}
	case 4:
		return []constructionColumn{col("Дата и смена", 29, "date", "shift"), col("Работы и их месторасположение", 56, "description"), col("Погода, температура", 26, "weather"), col("Меры в особых условиях", 30, "measures"), col("Приемка:\nед. изм.", 23, "unit"), col("Приемка:\nкол-во", 23, "quantity"), col("Операционный контроль, измерения", 43, "control"), col("Ответственный и оценивающий качество: должности, ФИО, подписи", 43, "persons", "_signature")}
	case 5:
		return []constructionColumn{col("Специальный журнал и дата выдачи", 100, "name", "issued"), col("Организация и ответственное лицо", 100, "organization"), col("Дата сдачи-приемки, должностное лицо, подпись", 73, "handed", "recipient", "_signature")}
	case 6:
		return []constructionColumn{col("Контролирующее лицо: ФИО, должность", 65, "inspector"), col("Дата", 28, "date"), col("Замечания / предписания", 100, "remark"), col("Принятие к исполнению и устранение", 80, "response")}
	default:
		return []constructionColumn{col("№ п/п", 16, "_number"), col("Контролирующий орган", 37, "authority"), col("Документ о назначении / допуске: номер и дата", 43, "order"), col("Должностное лицо: ФИО", 38, "inspector"), col("Начало и окончание", 30, "start", "end"), col("Подпись\nдолжн. лица", 26, "_signature"), col("Документ по результатам", 42, "result"), col("Примечание", 41, "note")}
	}
}

// Split tall rows across pages while repeating column headings. Each cell keeps
// its own line offset; one long description cannot overprint the footer.
func constructionGrid(p *fpdf.Fpdf, columns []constructionColumn, values []string, nextPage func(), header bool) {
	lines := make([][]string, len(columns))
	maximum := 1
	lineHeight := 4.5
	for i, col := range columns {
		value := values[i]
		if !header && slices.Contains(col.keys, "_signature") && col.keys[0] != "_signature" {
			value += "\n\nПодписи: ______________"
		}
		lines[i] = p.SplitText(value, col.width-4)
		if len(lines[i]) == 0 {
			lines[i] = []string{""}
		}
		if len(lines[i]) > maximum {
			maximum = len(lines[i])
		}
	}
	for offset := 0; offset < maximum; {
		_, pageH := p.GetPageSize()
		available := int(math.Floor((pageH - 17 - p.GetY() - 4) / lineHeight))
		if available < 1 && nextPage != nil {
			nextPage()
			continue
		}
		if available < 1 {
			available = 1
		}
		count := min(maximum-offset, available)
		height := float64(count)*lineHeight + 4
		y := p.GetY()
		x := 12.0
		p.SetDrawColor(130, 130, 130)
		p.SetLineWidth(.2)
		for i, col := range columns {
			if header {
				p.SetFillColor(238, 240, 242)
				p.Rect(x, y, col.width, height, "DF")
			} else {
				p.Rect(x, y, col.width, height, "D")
			}
			for j := 0; j < count && offset+j < len(lines[i]); j++ {
				p.SetXY(x+2, y+2+float64(j)*lineHeight)
				p.CellFormat(col.width-4, lineHeight, lines[i][offset+j], "", 0, "L", false, 0, "")
			}
			x += col.width
		}
		p.SetXY(12, y+height)
		offset += count
		if offset < maximum && nextPage != nil {
			nextPage()
		}
	}
}
