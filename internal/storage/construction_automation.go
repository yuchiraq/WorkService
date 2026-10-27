package storage

import (
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"project/internal/models"
)

type DocumentPerson struct {
	Name     string `json:"name"`
	Position string `json:"position"`
	Value    string `json:"value"`
	Assigned bool   `json:"assigned"`
}
type DocumentAutomation struct {
	Defaults    map[string]string   `json:"defaults"`
	People      []DocumentPerson    `json:"people"`
	Suggestions map[string][]string `json:"suggestions"`
}

func documentPerson(name, position string) string {
	return strings.TrimSpace(strings.TrimSpace(position) + " " + strings.TrimSpace(name))
}

// Reusable text comes from current saved document snapshots, so it participates
// in JSON/SQLite backup and migration without a second, divergent catalogue.
func ConstructionAutomation(object models.Object) (DocumentAutomation, error) {
	result := DocumentAutomation{Defaults: map[string]string{}, People: []DocumentPerson{}, Suggestions: map[string][]string{}}
	workers, err := GetWorkers()
	if err != nil {
		return result, err
	}
	users, err := GetUsers()
	if err != nil {
		return result, err
	}
	entries, err := GetTimesheets()
	if err != nil {
		return result, err
	}
	assigned := map[string]bool{}
	for _, e := range entries {
		if e.UserMark == "" && slices.Contains(e.ObjectIDs, object.ID) {
			for _, id := range e.WorkerIDs {
				assigned[id] = true
			}
		}
	}
	byUser := map[string]models.Worker{}
	seen := map[string]bool{}
	for _, w := range workers {
		if w.UserID != "" {
			byUser[w.UserID] = w
		}
		if w.IsFired || strings.TrimSpace(w.Name) == "" {
			continue
		}
		value := documentPerson(w.Name, w.Position)
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		result.People = append(result.People, DocumentPerson{Name: w.Name, Position: w.Position, Value: value, Assigned: assigned[w.ID] || w.UserID == object.ResponsibleUserID})
	}
	for _, u := range users {
		if u.ID == object.ResponsibleUserID {
			w := byUser[u.ID]
			result.Defaults["responsible"] = documentPerson(u.Name, w.Position)
			result.Defaults["contractor_rep"] = result.Defaults["responsible"]
		}
		if _, exists := byUser[u.ID]; exists || strings.TrimSpace(u.Name) == "" {
			continue
		}
		key := strings.ToLower(u.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		result.People = append(result.People, DocumentPerson{Name: u.Name, Value: u.Name, Assigned: u.ID == object.ResponsibleUserID})
	}
	constructionMutex.RLock()
	docs := append([]models.ConstructionDocument(nil), constructionDocuments...)
	constructionMutex.RUnlock()
	sort.Slice(docs, func(i, j int) bool {
		if (docs[i].ObjectID == object.ID) != (docs[j].ObjectID == object.ID) {
			return docs[i].ObjectID == object.ID
		}
		return docs[i].UpdatedAt > docs[j].UpdatedAt
	})
	keys := map[string]bool{}
	remember := func(key, value string) {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > 2000 {
			return
		}
		canonical := strings.ToLower(strings.Join(strings.Fields(value), " "))
		if keys[key+"\x00"+canonical] || len(result.Suggestions[key]) >= 40 {
			return
		}
		keys[key+"\x00"+canonical] = true
		result.Suggestions[key] = append(result.Suggestions[key], value)
	}
	for _, d := range docs {
		if d.Content.Archived {
			continue
		}
		if d.ObjectID == object.ID && d.Content.Kind == "requisites" {
			for k, v := range d.Content.Fields {
				result.Defaults[k] = v
			}
		}
		for k, v := range d.Content.Fields {
			if slices.Contains([]string{"work_name", "works", "project", "materials", "performer", "contractor", "customer", "designer", "responsible", "contractor_rep", "subcontractor_rep", "technical_rep", "author_rep", "quality_rep", "permit", "subcontractors"}, k) {
				remember(k, v)
			}
		}
		for _, r := range d.Content.Rows {
			for k, v := range r.Fields {
				if slices.Contains([]string{"person", "crew", "description", "measures", "unit", "control", "persons", "name", "organization", "inspector", "remark", "response", "authority"}, k) {
					remember(k, v)
				}
			}
		}
	}
	// The object's current responsible person takes priority for new drafts.
	for _, u := range users {
		if u.ID == object.ResponsibleUserID {
			result.Defaults["responsible"] = documentPerson(u.Name, byUser[u.ID].Position)
			break
		}
	}
	result.Defaults["object_name"], result.Defaults["address"] = object.Name, object.Address
	if result.Defaults["contractor_rep"] == "" {
		result.Defaults["contractor_rep"] = result.Defaults["responsible"]
	}
	c := collate.New(language.Russian)
	sort.SliceStable(result.People, func(i, j int) bool {
		a, b := result.People[i], result.People[j]
		if a.Assigned != b.Assigned {
			return a.Assigned
		}
		return c.CompareString(a.Name, b.Name) < 0
	})
	return result, nil
}

func ConstructionAssignmentRows(object models.Object, from, to string, section int) ([]models.ConstructionRow, error) {
	entries, err := GetTimesheets()
	if err != nil {
		return nil, err
	}
	workers, err := GetWorkers()
	if err != nil {
		return nil, err
	}
	auto, err := ConstructionAutomation(object)
	if err != nil {
		return nil, err
	}
	byID := map[string]models.Worker{}
	for _, w := range workers {
		byID[w.ID] = w
	}
	rows := []models.ConstructionRow{}
	groups := map[string]int{}
	crewIDs := map[int]map[string]bool{}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Date == entries[j].Date {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].Date < entries[j].Date
	})
	for _, e := range entries {
		if e.UserMark != "" || e.Date < from || e.Date > to || !slices.Contains(e.ObjectIDs, object.ID) {
			continue
		}
		if section == 2 {
			for _, id := range e.WorkerIDs {
				w, ok := byID[id]
				if !ok {
					continue
				}
				position := strings.ToLower(w.Position)
				engineering := w.UserID == object.ResponsibleUserID
				for _, word := range []string{"инженер", "прораб", "мастер", "начальник", "руководитель"} {
					engineering = engineering || strings.Contains(position, word)
				}
				if !engineering {
					continue
				}
				key := "staff:" + id
				if _, ok := groups[key]; ok {
					continue
				}
				groups[key] = len(rows)
				rows = append(rows, models.ConstructionRow{ID: uuid.NewString(), Section: 2, Fields: map[string]string{"person": documentPerson(w.Name, w.Position), "start": e.Date}, Sources: []string{key}})
			}
			continue
		}
		key := e.Date + "\x00" + e.StartTime + "\x00" + e.EndTime
		index, ok := groups[key]
		if !ok {
			index = len(rows)
			groups[key] = index
			rows = append(rows, models.ConstructionRow{ID: uuid.NewString(), Section: 4, Fields: map[string]string{"date": e.Date, "shift": e.StartTime + " - " + e.EndTime, "persons": documentResponsible(auto.Defaults)}})
		}
		r := &rows[index]
		if crewIDs[index] == nil {
			crewIDs[index] = map[string]bool{}
		}
		r.Sources = append(r.Sources, "assignment:"+e.ID)
		if len(e.ObjectIDs) == 1 && strings.TrimSpace(e.Notes) != "" {
			r.Fields["description"] = appendDistinctLine(r.Fields["description"], strings.TrimSpace(e.Notes))
		}
		for _, id := range e.WorkerIDs {
			if _, ok := byID[id]; ok {
				crewIDs[index][id] = true
			}
		}
	}
	for index, ids := range crewIDs {
		members := []models.Worker{}
		for id := range ids {
			members = append(members, byID[id])
		}
		collator := collate.New(language.Russian)
		sort.Slice(members, func(i, j int) bool { return collator.CompareString(members[i].Name, members[j].Name) < 0 })
		rows[index].Fields["crew"] = ""
		for _, w := range members {
			rows[index].Fields["crew"] = appendDistinctLine(rows[index].Fields["crew"], documentPerson(w.Name, w.Position))
		}
	}
	return rows, nil
}

func documentResponsible(defaults map[string]string) string {
	return appendDistinctLine(defaults["responsible"], defaults["quality_rep"])
}
func appendDistinctLine(value, line string) string {
	line = strings.TrimSpace(line)
	if line == "" || slices.Contains(strings.Split(value, "\n"), line) {
		return value
	}
	if value == "" {
		return line
	}
	return value + "\n" + line
}
