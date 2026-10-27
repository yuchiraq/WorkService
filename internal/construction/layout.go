package construction

// Block describes the fixed wording and the position of variable information.
// Browser and PDF use this shared layout, not two independently maintained forms.
type Block struct {
	Kind     string `json:"kind"`
	Text     string `json:"text,omitempty"`
	Key      string `json:"key,omitempty"`
	Hint     string `json:"hint,omitempty"`
	Optional bool   `json:"optional,omitempty"`
}

var ActLayout = []Block{
	{Kind: "reference", Text: "Приложение Л\nк СН 1.03.04-2020"},
	{Kind: "title", Text: "АКТ"},
	{Kind: "subtitle", Text: "освидетельствования скрытых работ"},
	{Kind: "field", Key: "work_name", Hint: "наименование работ"},
	{Kind: "field", Text: "выполненных в", Key: "object_name", Hint: "наименование объекта"},
	{Kind: "field", Key: "address", Hint: "место расположения объекта"},
	{Kind: "date", Key: "act_date", Text: "Дата освидетельствования"},
	{Kind: "text", Text: "Комиссия в составе:"},
	{Kind: "field", Text: "представителя подрядчика", Key: "contractor_rep", Hint: "должность, фамилия, инициалы"},
	{Kind: "field", Text: "представителя субподрядной организации (в случаях выполнения работ субподрядной организацией)", Key: "subcontractor_rep", Hint: "должность, фамилия, инициалы"},
	{Kind: "field", Text: "представителя технического надзора", Key: "technical_rep", Hint: "должность, фамилия, инициалы"},
	{Kind: "field", Text: "представителя авторского надзора", Key: "author_rep", Hint: "должность, фамилия, инициалы / основание отсутствия"},
	{Kind: "field", Text: "произвела осмотр работ, выполненных", Key: "performer", Hint: "наименование строительно-монтажной организации"},
	{Kind: "text", Text: "и составила настоящий акт о нижеследующем:"},
	{Kind: "field", Text: "1 К освидетельствованию предъявлены следующие работы", Key: "works", Hint: "наименование работ, оси, отметки, этаж, помещения"},
	{Kind: "field", Text: "2 Работы выполнены по проектной документации", Key: "project", Hint: "разработчик проектной документации, номера чертежей и даты их составления"},
	{Kind: "field", Text: "3 При выполнении работ применены", Key: "materials", Hint: "материалы, конструкции, изделия со ссылками на сертификаты и другие документы о качестве"},
	{Kind: "violations", Text: "4 При выполнении работ", Key: "violations", Hint: "нарушения требований ТНПА и (или) проектной документации"},
	{Kind: "field", Key: "deviations", Text: "При наличии нарушений", Hint: "кем согласованы работы, номера чертежей и даты согласования", Optional: true},
	{Kind: "date", Text: "5 Даты: начало работ", Key: "start_date"},
	{Kind: "date", Text: "окончание работ", Key: "end_date"},
	{Kind: "signatures"},
	{Kind: "field", Text: "Дополнительные сведения и ссылки на приложения", Key: "attachments", Optional: true},
}

// The outer title is a separate sheet, not part of section 1.
var JournalTitleLayout = []Block{
	{Kind: "reference", Text: "СН 1.03.04-2020"},
	{Kind: "title", Text: "ЖУРНАЛ"},
	{Kind: "subtitle", Text: "производства работ"},
	{Kind: "cover-summary"},
}

var CoverLayout = []Block{
	{Kind: "reference", Text: "Приложение Б\nк СН 1.03.04-2020"},
	{Kind: "subtitle", Text: "Раздел 1\nОбщие сведения"},
	{Kind: "field", Key: "contractor", Hint: "наименование генерального подрядчика (подрядчика) или застройщика при строительстве собственными силами"},
	{Kind: "field", Text: "Наименование объекта строительства", Key: "object_name"},
	{Kind: "field", Text: "Местонахождение объекта строительства", Key: "address"},
	{Kind: "field", Text: "Разрешение на производство строительно-монтажных работ или уведомление о проведении строительных работ", Key: "permit", Hint: "дата, номер, наименование выдавшего органа / дата и номер уведомления"},
	{Kind: "field", Text: "Лицо, ответственное за строительство объекта и ведение журнала производства работ", Key: "responsible", Hint: "должность, фамилия, имя, отчество"},
	{Kind: "signature-line"},
	{Kind: "field", Text: "Генеральная проектная организация, главный инженер проекта", Key: "designer", Hint: "наименование организации, фамилия, имя, отчество ГИП"},
	{Kind: "signature-line"},
	{Kind: "field", Text: "Заказчик, застройщик", Key: "customer", Hint: "организация, должность, фамилия, имя, отчество руководителя"},
	{Kind: "signature-line"},
	{Kind: "field", Text: "Технический надзор", Key: "technical_rep", Hint: "организация, должность, фамилия, имя, отчество представителя"},
	{Kind: "signature-line"},
	{Kind: "text", Text: "Начало работ:"},
	{Kind: "date", Text: "по плану (договору)", Key: "planned_start"},
	{Kind: "date", Text: "фактически", Key: "actual_start"},
	{Kind: "text", Text: "Окончание работ (ввод в эксплуатацию):"},
	{Kind: "date", Text: "по плану (договору)", Key: "planned_end"},
	{Kind: "date", Text: "фактически", Key: "actual_end"},
	{Kind: "text", Text: "В настоящем журнале ______ пронумерованных и прошнурованных страниц."},
	{Kind: "field", Text: "Руководитель организации, выдавшей журнал", Key: "issuer", Hint: "должность, фамилия, имя, отчество"},
	{Kind: "signature-line"},
	{Kind: "date", Text: "Дата выдачи журнала", Key: "issue_date"},
	{Kind: "text", Text: "Печать организации (при её применении) ____________________"},
	{Kind: "field", Text: "Основные показатели объекта строительства и сметная стоимость", Key: "indicators", Hint: "мощность, производительность, полезная площадь, вместимость и т. п."},
	{Kind: "field", Text: "Утверждение проектной документации", Key: "approval", Hint: "наименование органа, дата и номер постановления, решения или приказа"},
	{Kind: "field", Text: "Субподрядные организации и выполняемые ими работы", Key: "subcontractors"},
	{Kind: "field", Text: "Субподрядные проектные организации, выполнившие разделы проекта", Key: "subdesigners"},
	{Kind: "field", Text: "Отметки об изменениях в записях данного раздела", Key: "cover_changes"},
}

type Signatory struct {
	Role           string `json:"role"`
	Representative string `json:"representative"`
	Name           string `json:"name"`
	At             string `json:"at"`
}

var Signatories = []Signatory{
	{"Представитель подрядчика", "contractor_rep", "contractor_signer", "contractor_signed_at"},
	{"Представитель субподрядной организации (при её участии)", "subcontractor_rep", "subcontractor_signer", "subcontractor_signed_at"},
	{"Представитель технического надзора", "technical_rep", "technical_signer", "technical_signed_at"},
	{"Представитель авторского надзора", "author_rep", "author_signer", "author_signed_at"},
}

type Column struct {
	Label string   `json:"label"`
	Keys  []string `json:"keys"`
	Width float64  `json:"width"`
}

func Columns(section int) []Column {
	c := func(label string, width float64, keys ...string) Column { return Column{label, keys, width} }
	switch section {
	case 2:
		return []Column{c("Фамилия, имя, отчество, должность, участок работ", 88, "person"), c("Дата начала работ", 35, "start"), c("Разрешение на право производства работ / аттестация", 99, "qualification"), c("Дата окончания работ", 35, "end")}
	case 3:
		return []Column{c("Наименование акта, номер, расположение конструкций и работ", 139, "act"), c("Дата подписания, фамилии, инициалы и должности подписавших", 118, "signed_date", "signers")}
	case 4:
		return []Column{c("Дата и смена", 28, "date", "shift"), c("Работы и их месторасположение; бригада", 48, "description", "crew"), c("Погода, температура воздуха", 28, "weather", "weather_source"), c("Меры в особых условиях", 28, "measures"), c("Приёмка\nед. изм.", 20, "unit"), c("Приёмка\nкол-во", 20, "quantity"), c("Операционный контроль, измерения", 42, "control"), c("Ответственный и оценивающий качество: должности, ФИО, подписи", 43, "persons", "_signature")}
	case 5:
		return []Column{c("Специальный журнал и дата выдачи", 93, "name", "issued"), c("Организация и ответственное лицо", 93, "organization"), c("Дата сдачи-приёмки, должностное лицо, подпись", 71, "handed", "recipient", "_signature")}
	case 6:
		return []Column{c("Контролирующее лицо: ФИО, должность", 60, "inspector"), c("Дата", 27, "date"), c("Замечание / ссылка на предписание", 95, "remark"), c("Принятие к исполнению и устранение", 75, "response")}
	default:
		return []Column{c("№ п/п", 14, "_number"), c("Контролирующий орган", 34, "authority"), c("Документ о назначении / допуске: вид, номер, дата", 41, "order"), c("Должностное лицо: ФИО", 36, "inspector"), c("Начало и окончание", 29, "start", "end"), c("Подпись должн. лица", 24, "_signature"), c("Документ по результатам", 40, "result"), c("Примечание", 39, "note")}
	}
}

func SectionTitle(section int) string {
	switch section {
	case 2:
		return "Список инженерно-технического персонала, занятого на строительстве объекта"
	case 3:
		return "Перечень актов промежуточной приемки ответственных конструкций и освидетельствования скрытых работ"
	case 4:
		return "Сведения о производстве строительно-монтажных работ с учетом результатов операционного контроля их качества"
	case 5:
		return "Перечень специальных журналов работ"
	case 6:
		return "Замечания лиц, контролирующих производство и безопасность работ"
	default:
		return "Сведения о мероприятиях технического (технологического, поверочного) характера в рамках контроля (надзора) соблюдения обязательных требований ТНПА при строительстве"
	}
}
