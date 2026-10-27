// Package construction defines the Belarusian document fields shared by the
// editor, validation and print renderer (SN 1.03.04-2020, amendment 1).
package construction

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"project/internal/models"
)

const Basis = "СН 1.03.04-2020, изменение № 1 (переиздание 2024): приложения Б и Л"

type Field struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}
type Section struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Fields []Field `json:"fields"`
}

func f(key, label string, required bool) Field    { return Field{key, label, "text", required} }
func long(key, label string, required bool) Field { return Field{key, label, "textarea", required} }
func date(key, label string, required bool) Field { return Field{key, label, "date", required} }

var Common = []Field{
	f("object_name", "Наименование объекта", true),
	long("address", "Местонахождение объекта", true),
	f("contractor", "Генеральный подрядчик / подрядчик", true),
	f("contractor_rep", "Представитель подрядчика: должность, ФИО", true),
	f("subcontractor_rep", "Представитель субподрядчика: организация, должность, ФИО", false),
	f("customer", "Заказчик / застройщик: организация, руководитель", true),
	f("technical_rep", "Технический надзор: организация, должность, ФИО", true),
	f("author_rep", "Авторский надзор: организация, должность, ФИО", false),
	f("designer", "Генеральная проектная организация, ГИП", true),
	long("project", "Проект: разработчик, номера и даты чертежей", true),
	long("permit", "Разрешение / уведомление: дата, номер, орган", true),
	f("responsible", "Ответственный за строительство и ведение журнала: должность, ФИО", true),
	f("quality_rep", "Внутренний контроль качества: должность, ФИО", false),
	{Key: "weather_city", Label: "Город для погоды", Type: "city"},
	{Key: "weather_latitude", Label: "Широта", Type: "hidden"},
	{Key: "weather_longitude", Label: "Долгота", Type: "hidden"},
}

var Act = []Field{
	f("work_name", "Наименование скрытых работ", true),
	date("act_date", "Дата освидетельствования", true),
	long("works", "1. Предъявленные работы и их расположение (оси, отметки, помещения)", true),
	long("project", "2. Проект: разработчик, номера и даты чертежей", true),
	long("materials", "3. Материалы, конструкции, изделия; сертификаты / документы о качестве", true),
	{Key: "violations", Label: "4. Нарушения требований ТНПА / проекта", Type: "violations", Required: true},
	long("deviations", "При нарушениях: согласование, номера чертежей и даты", false),
	date("start_date", "5. Начало работ", true),
	date("end_date", "Окончание работ", true),
	f("performer", "Организация, выполнившая работы", true),
	f("object_name", "Наименование объекта", true),
	long("address", "Местонахождение объекта", true),
	f("contractor_rep", "Представитель подрядчика: должность, ФИО", true),
	f("subcontractor_rep", "Представитель субподрядчика (если участвовал)", false),
	f("technical_rep", "Представитель технического надзора: должность, ФИО", true),
	f("author_rep", "Представитель авторского надзора: должность, ФИО / основание отсутствия", true),
	long("attachments", "Дополнительные сведения и ссылки на приложения", false),
	f("contractor_signer", "Подрядчик: фамилия, инициалы подписанта", false),
	{Key: "contractor_signed_at", Label: "Подрядчик: дата и время подписания", Type: "datetime-local"},
	f("subcontractor_signer", "Субподрядчик: фамилия, инициалы подписанта", false),
	{Key: "subcontractor_signed_at", Label: "Субподрядчик: дата и время подписания", Type: "datetime-local"},
	f("technical_signer", "Технадзор: фамилия, инициалы подписанта", false),
	{Key: "technical_signed_at", Label: "Технадзор: дата и время подписания", Type: "datetime-local"},
	f("author_signer", "Авторский надзор: фамилия, инициалы подписанта", false),
	{Key: "author_signed_at", Label: "Авторский надзор: дата и время подписания", Type: "datetime-local"},
}

var JournalCover = []Field{
	f("object_name", "Наименование объекта", true), long("address", "Местонахождение объекта", true),
	f("contractor", "Генеральный подрядчик / застройщик при строительстве своими силами", true),
	long("permit", "Разрешение / уведомление: дата, номер, орган", true),
	f("responsible", "Ответственный за строительство и ведение журнала: должность, ФИО", true),
	f("designer", "Генеральная проектная организация, ГИП", true),
	f("customer", "Заказчик / застройщик: организация, должность, ФИО руководителя", true),
	f("technical_rep", "Технический надзор: организация, должность, ФИО", true),
	date("planned_start", "Начало работ по плану / договору", false), date("actual_start", "Фактическое начало работ", false),
	date("planned_end", "Окончание по плану / договору", false), date("actual_end", "Фактическое окончание / ввод", false),
	f("issuer", "Руководитель организации, выдавшей журнал: должность, ФИО", true), date("issue_date", "Дата выдачи журнала", true),
	long("indicators", "Основные показатели объекта и сметная стоимость", false),
	long("approval", "Утверждение проекта: орган, дата, номер решения", false),
	long("subcontractors", "Субподрядные организации и выполняемые работы", false),
	long("subdesigners", "Субподрядные проектные организации и разделы проекта", false),
	long("cover_changes", "Отметки об изменениях в разделе 1", false),
}

var JournalSections = []Section{
	{2, "Инженерно-технический персонал", []Field{long("person", "ФИО, должность, участок работ", true), date("start", "Дата начала работ", true), long("qualification", "Разрешение на право производства работ / аттестация", true), date("end", "Дата окончания работ", false)}},
	{3, "Реестр актов", []Field{long("act", "Наименование и номер акта, расположение конструкций и работ", true), date("signed_date", "Дата фактического подписания", true), long("signers", "Фамилии, инициалы и должности подписавших", true)}},
	{4, "Производство работ", []Field{date("date", "Дата производства работ", true), f("shift", "Смена", true), long("description", "Работы и их расположение", true), long("crew", "Рабочие / бригада по назначениям", false), f("weather", "Погода и температура наружного воздуха", true), {Key: "weather_source", Label: "Источник погоды", Type: "annotation"}, {Key: "weather_date", Label: "Дата данных погоды", Type: "hidden"}, long("measures", "Меры в особых условиях (зима, жара и др.)", true), f("unit", "Единица измерения", true), f("quantity", "Количество принятых работ", true), long("control", "Результаты операционного контроля, параметры измерений", true), long("persons", "Ответственный и оценивающий качество: должности, ФИО", true)}},
	{5, "Специальные журналы", []Field{f("name", "Наименование специального журнала", true), date("issued", "Дата выдачи", true), long("organization", "Организация, ответственное лицо: должность, ФИО", true), date("handed", "Дата сдачи-приемки", false), f("recipient", "Принявшее должностное лицо", false)}},
	{6, "Замечания и устранение", []Field{f("inspector", "Контролирующее лицо: должность, ФИО", true), date("date", "Дата", true), long("remark", "Замечание / ссылка на предписание", true), long("response", "Принятие к исполнению и устранение", false)}},
	{7, "Контроль и надзор", []Field{f("authority", "Контролирующий / надзорный орган", true), long("order", "Документ о назначении мероприятия / допуске: вид, номер, дата", false), f("inspector", "Должностное лицо: ФИО", true), date("start", "Начало мероприятия", true), date("end", "Окончание мероприятия", false), long("result", "Документ по результатам мероприятия", false), long("note", "Примечание, в том числе замена должностного лица", false)}},
}

func Fields(kind string) []Field {
	switch kind {
	case "requisites":
		return Common
	case "act":
		return Act
	case "journal":
		return JournalCover
	}
	return nil
}
func RowFields(section int) []Field {
	for _, s := range JournalSections {
		if s.Number == section {
			return s.Fields
		}
	}
	return nil
}
func ValidDate(v string) bool {
	d, e := time.Parse("2006-01-02", v)
	return e == nil && d.Year() >= 1900 && d.Year() <= 2200
}

func validateFields(values map[string]string, schema []Field) error {
	allowed := map[string]Field{}
	for _, f := range schema {
		allowed[f.Key] = f
	}
	for k, v := range values {
		f, ok := allowed[k]
		if !ok || !utf8.ValidString(v) || utf8.RuneCountInString(v) > 5000 || strings.ContainsAny(v, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c") {
			return fmt.Errorf("Недопустимое поле или слишком длинный текст: %s", k)
		}
		if strings.HasSuffix(k, "_signer") && utf8.RuneCountInString(v) > 150 {
			return fmt.Errorf("Фамилия и инициалы: не более 150 символов")
		}
		if v != "" && f.Type == "date" && !ValidDate(v) {
			return fmt.Errorf("Проверьте дату: %s", f.Label)
		}
		if v != "" && f.Type == "datetime-local" {
			d, err := time.Parse("2006-01-02T15:04", v)
			if err != nil || d.Year() < 1900 || d.Year() > 2200 {
				return fmt.Errorf("Проверьте дату и время: %s", f.Label)
			}
		}
		if f.Type == "violations" && v != "" && v != "none" && v != "present" {
			return fmt.Errorf("Укажите результат проверки нарушений")
		}
	}
	for _, pair := range [][2]string{{"start_date", "end_date"}, {"start", "end"}, {"actual_start", "actual_end"}, {"planned_start", "planned_end"}} {
		if values[pair[0]] != "" && values[pair[1]] != "" && values[pair[0]] > values[pair[1]] {
			return fmt.Errorf("Дата окончания не может быть раньше начала")
		}
	}
	if values["act_date"] != "" && values["end_date"] > values["act_date"] {
		return fmt.Errorf("Дата освидетельствования не может быть раньше окончания работ")
	}
	if values["weather_date"] != "" && (!ValidDate(values["weather_date"]) || values["weather_date"] != values["date"]) {
		return fmt.Errorf("Погода относится к другой дате. Обновите её или удалите автоматическую подстановку")
	}
	if values["weather_latitude"] != "" || values["weather_longitude"] != "" {
		for key, limit := range map[string]float64{"weather_latitude": 90, "weather_longitude": 180} {
			v, e := strconv.ParseFloat(values[key], 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < -limit || v > limit {
				return fmt.Errorf("Выберите город из списка найденных населённых пунктов")
			}
		}
		if strings.TrimSpace(values["weather_city"]) == "" {
			return fmt.Errorf("Укажите город для погоды")
		}
	}
	return nil
}

func Validate(c models.ConstructionContent) error {
	if Fields(c.Kind) == nil {
		return fmt.Errorf("Неизвестный вид документа")
	}
	if utf8.RuneCountInString(c.Number) > 80 || strings.ContainsAny(c.Number, "\r\n\x00") {
		return fmt.Errorf("Номер документа: не более 80 символов")
	}
	if err := validateFields(c.Fields, Fields(c.Kind)); err != nil {
		return err
	}
	if len(c.Rows) > 1500 || (c.Kind != "journal" && len(c.Rows) != 0) {
		return fmt.Errorf("В журнале допускается до 1500 записей; создайте следующий том")
	}
	ids := map[string]bool{}
	for _, r := range c.Rows {
		if r.ID == "" || len(r.ID) > 80 || ids[r.ID] || RowFields(r.Section) == nil {
			return fmt.Errorf("Некорректная или повторяющаяся запись журнала")
		}
		ids[r.ID] = true
		if len(r.Sources) > 500 {
			return fmt.Errorf("Слишком много источников записи")
		}
		for _, id := range r.Sources {
			if len(id) > 100 {
				return fmt.Errorf("Некорректный источник записи")
			}
		}
		if err := validateFields(r.Fields, RowFields(r.Section)); err != nil {
			return err
		}
		if r.Reviewed {
			if missing := Missing(r.Fields, RowFields(r.Section)); len(missing) > 0 {
				return fmt.Errorf("Для проверки записи заполните: %s", strings.Join(missing, "; "))
			}
		}
	}
	return nil
}

func Missing(values map[string]string, fields []Field) []string {
	var result []string
	for _, f := range fields {
		if f.Required && strings.TrimSpace(values[f.Key]) == "" {
			result = append(result, f.Label)
		}
	}
	if values["violations"] == "present" && strings.TrimSpace(values["deviations"]) == "" {
		result = append(result, "Согласование нарушений")
	}
	return result
}
