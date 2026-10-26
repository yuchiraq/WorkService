package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"project/internal/construction"
	"project/internal/models"
)

var (
	constructionMutex     sync.RWMutex
	constructionDocuments []models.ConstructionDocument
	ErrDocumentConflict   = errors.New("Документ уже изменён. Ваш текст сохранён в редакторе. Откройте актуальную версию в другой вкладке и перенесите изменения")
	ErrDocumentNotFound   = errors.New("Документ не найден на этом объекте")
)

const constructionFile = "storage/construction_documents.json"

func LoadConstructionDocuments() error {
	constructionMutex.Lock()
	defer constructionMutex.Unlock()
	data, err := readData(constructionFile)
	if os.IsNotExist(err) {
		constructionDocuments = nil
		return nil
	}
	if err != nil {
		return err
	}
	var docs []models.ConstructionDocument
	if err = json.Unmarshal(data, &docs); err != nil {
		return err
	}
	if err = ValidateConstructionDocuments(docs); err != nil {
		return err
	}
	constructionDocuments = docs
	return nil
}

func ValidateConstructionDocuments(docs []models.ConstructionDocument) error {
	ids, numbers := map[string]bool{}, map[string]bool{}
	for _, d := range docs {
		key := d.ObjectID + "\x00" + d.Content.Kind + "\x00" + strings.ToLower(strings.TrimSpace(d.Content.Number))
		if d.ID == "" || d.ObjectID == "" || ids[d.ID] || numbers[key] || d.Version < 1 || len(d.History) == 0 {
			return fmt.Errorf("construction documents: invalid identity, number or version")
		}
		ids[d.ID], numbers[key] = true, true
		if err := construction.Validate(d.Content); err != nil {
			return err
		}
		if len(d.History) != int(d.Version) {
			return fmt.Errorf("construction documents: missing revision")
		}
		for i, rev := range d.History {
			if rev.Version != int64(i+1) || rev.Content.Kind != d.Content.Kind {
				return fmt.Errorf("construction documents: invalid revision")
			}
			if _, err := time.Parse(time.RFC3339Nano, rev.At); err != nil {
				return err
			}
			if err := construction.Validate(rev.Content); err != nil {
				return err
			}
		}
		current, _ := json.Marshal(d.Content)
		last, _ := json.Marshal(d.History[len(d.History)-1].Content)
		if string(current) != string(last) {
			return fmt.Errorf("construction documents: snapshot mismatch")
		}
	}
	return nil
}

func cloneConstruction[T any](value T) T {
	data, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}

func GetConstructionDocuments(objectID string) []models.ConstructionDocument {
	constructionMutex.RLock()
	defer constructionMutex.RUnlock()
	result := []models.ConstructionDocument{}
	for _, d := range constructionDocuments {
		if d.ObjectID == objectID {
			d.History = nil
			result = append(result, d)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt > result[j].UpdatedAt })
	return cloneConstruction(result)
}

func GetConstructionDocument(objectID, id string) (models.ConstructionDocument, error) {
	constructionMutex.RLock()
	defer constructionMutex.RUnlock()
	for _, d := range constructionDocuments {
		if d.ObjectID == objectID && d.ID == id {
			return cloneConstruction(d), nil
		}
	}
	return models.ConstructionDocument{}, ErrDocumentNotFound
}

func SaveConstructionDocument(objectID, id string, version int64, content models.ConstructionContent, actorID, actor, reason string) (models.ConstructionDocument, error) {
	constructionMutex.Lock()
	defer constructionMutex.Unlock()
	doc := models.ConstructionDocument{}
	index := -1
	for i, d := range constructionDocuments {
		if d.ObjectID == objectID && d.ID == id {
			doc, index = d, i
			break
		}
	}
	if id != "" && index < 0 {
		return doc, ErrDocumentNotFound
	}
	if version != doc.Version {
		return doc, ErrDocumentConflict
	}
	if index >= 0 && content.Kind != doc.Content.Kind {
		return doc, fmt.Errorf("Вид документа нельзя изменить")
	}
	if content.Kind == "requisites" {
		content.Number = "Реквизиты"
		content.Archived = false
	}
	content.Number = strings.TrimSpace(content.Number)
	if id == "" {
		if content.Archived {
			return doc, fmt.Errorf("Новый документ не может быть архивным")
		}
		if content.Number == "" {
			prefix := "АСР"
			if content.Kind == "journal" {
				prefix = "ЖР"
			}
			for n := 1; ; n++ {
				content.Number = fmt.Sprintf("%s-%03d", prefix, n)
				used := false
				for _, other := range constructionDocuments {
					if other.ObjectID == objectID && other.Content.Kind == content.Kind && strings.EqualFold(other.Content.Number, content.Number) {
						used = true
						break
					}
				}
				if !used {
					break
				}
			}
		}
		doc.ID, doc.ObjectID = uuid.NewString(), objectID
	} else if content.Number == "" {
		return doc, fmt.Errorf("Укажите номер документа")
	}
	for _, other := range constructionDocuments {
		if other.ID != doc.ID && other.ObjectID == objectID && other.Content.Kind == content.Kind && strings.EqualFold(other.Content.Number, content.Number) {
			return doc, fmt.Errorf("Такой номер уже используется на объекте")
		}
	}
	if err := construction.Validate(content); err != nil {
		return doc, err
	}
	content = cloneConstruction(content)
	dateKey := map[int]string{2: "start", 3: "signed_date", 4: "date", 5: "issued", 6: "date", 7: "start"}
	sort.SliceStable(content.Rows, func(i, j int) bool {
		a, b := content.Rows[i], content.Rows[j]
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		x, y := a.Fields[dateKey[a.Section]], b.Fields[dateKey[b.Section]]
		if x == "" {
			return false
		}
		if y == "" {
			return true
		}
		return x < y
	})
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 500 {
		return doc, fmt.Errorf("Основание изменения: не более 500 символов")
	}
	if index >= 0 && reason == "" {
		return doc, fmt.Errorf("Укажите основание изменения")
	}
	if index < 0 {
		reason = "Создание документа"
	}
	doc.Version++
	doc.Content = content
	doc.UpdatedAt, doc.UpdatedBy = time.Now().UTC().Format(time.RFC3339Nano), actor
	doc.History = append(append([]models.ConstructionRevision(nil), doc.History...), models.ConstructionRevision{Version: doc.Version, At: doc.UpdatedAt, ActorID: actorID, Actor: actor, Reason: reason, Content: doc.Content})
	next := append([]models.ConstructionDocument(nil), constructionDocuments...)
	if index < 0 {
		next = append(next, doc)
	} else {
		next[index] = doc
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return models.ConstructionDocument{}, err
	}
	if err = writeData(constructionFile, data); err != nil {
		return models.ConstructionDocument{}, fmt.Errorf("Не удалось сохранить документ: %w", err)
	}
	constructionDocuments = next
	return cloneConstruction(doc), nil
}
