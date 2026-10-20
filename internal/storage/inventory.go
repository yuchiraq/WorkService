package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"project/internal/models"

	"github.com/google/uuid"
)

const MaxInventoryQuantity int64 = 1_000_000_000

var (
	ErrInventoryInvalid   = errors.New("Проверьте данные позиции")
	ErrInventoryNotFound  = errors.New("Позиция уже удалена. Обновите список")
	ErrInventoryConflict  = errors.New("Позиция уже изменена другим пользователем. Обновите список и повторите изменение")
	ErrInventoryDuplicate = errors.New("Такая позиция уже есть на объекте. Измените её количество")
	inventoryMutex        sync.RWMutex
	inventoryFile         = "storage/inventory.json"
	inventory             inventoryData
)

type inventoryData struct {
	Catalog []models.InventoryName   `json:"catalog"`
	Items   []models.InventoryItem   `json:"items"`
	History []models.InventoryChange `json:"history"`
}

type InventorySnapshot struct {
	Catalog []models.InventoryName   `json:"catalog"`
	Items   []models.InventoryItem   `json:"items"`
	History []models.InventoryChange `json:"history"`
}

type InventoryMutation struct {
	Action, ItemID, Name, Unit, Note string
	Quantity, Delta, Version         int64
	Material                         *string
}

// Legacy items are grouped at read time; migration keeps the original bytes intact.
func InventoryMaterial(name, material string) string {
	aliases := map[string]string{
		"нерж": "Нержавейка", "нержавейка": "Нержавейка", "нержавеющая": "Нержавейка",
		"пвх": "ПВХ", "pvc": "ПВХ", "оцинк": "Оцинковка", "оцинковка": "Оцинковка", "оцинкованная": "Оцинковка",
	}
	material = strings.Join(strings.Fields(material), " ")
	if material != "" {
		if canonical := aliases[strings.ToLower(strings.TrimSuffix(material, "."))]; canonical != "" {
			return canonical
		}
		return material
	}
	result := ""
	for _, word := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if match := aliases[word]; match != "" {
			if result != "" && result != match {
				return ""
			}
			result = match
		}
	}
	return result
}

func LoadInventory() error {
	inventoryMutex.Lock()
	defer inventoryMutex.Unlock()
	data, err := readData(inventoryFile)
	if os.IsNotExist(err) {
		inventory = inventoryData{}
		return nil
	}
	if err != nil {
		return err
	}
	var loaded inventoryData
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	inventory = loaded
	return nil
}

func ParseInventoryQuantity(value string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parts := strings.Split(value, ".")
	invalid := fmt.Errorf("%w: количество от 0 до 1 000 000, не более трёх знаков после запятой", ErrInventoryInvalid)
	if len(parts) > 2 || parts[0] == "" || len(parts[0]) > 7 {
		return 0, invalid
	}
	for _, part := range parts {
		if part == "" {
			return 0, invalid
		}
		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return 0, invalid
			}
		}
	}
	whole, _ := strconv.ParseInt(parts[0], 10, 64)
	quantity := whole * 1000
	if len(parts) == 2 {
		if len(parts[1]) > 3 {
			return 0, invalid
		}
		fraction, _ := strconv.ParseInt(parts[1]+strings.Repeat("0", 3-len(parts[1])), 10, 64)
		quantity += fraction
	}
	if quantity > MaxInventoryQuantity {
		return 0, invalid
	}
	return quantity, nil
}

func validInventoryUnit(unit string) bool {
	switch unit {
	case "pcs", "set", "pack", "m", "m2", "m3", "kg", "l":
		return true
	}
	return false
}

func sameInventoryName(name, unit, otherName, otherUnit string) bool {
	return strings.EqualFold(name, otherName) && unit == otherUnit
}

func GetInventory(objectID string) InventorySnapshot {
	inventoryMutex.RLock()
	defer inventoryMutex.RUnlock()
	result := InventorySnapshot{Catalog: append([]models.InventoryName{}, inventory.Catalog...), Items: []models.InventoryItem{}, History: []models.InventoryChange{}}
	for i := range result.Catalog {
		result.Catalog[i].Material = InventoryMaterial(result.Catalog[i].Name, result.Catalog[i].Material)
	}
	for _, item := range inventory.Items {
		if item.ObjectID == objectID {
			item.Material = InventoryMaterial(item.Name, item.Material)
			result.Items = append(result.Items, item)
		}
	}
	for i := len(inventory.History) - 1; i >= 0 && len(result.History) < 30; i-- {
		change := inventory.History[i]
		if change.ObjectID == objectID {
			// Snapshots must not expose pointers into the live storage state.
			if change.Before != nil {
				before := *change.Before
				change.Before = &before
			}
			if change.After != nil {
				after := *change.After
				change.After = &after
			}
			result.History = append(result.History, change)
		}
	}
	sort.SliceStable(result.Catalog, func(i, j int) bool {
		return strings.ToLower(result.Catalog[i].Name)+result.Catalog[i].Unit < strings.ToLower(result.Catalog[j].Name)+result.Catalog[j].Unit
	})
	sort.SliceStable(result.Items, func(i, j int) bool {
		return strings.ToLower(result.Items[i].Name)+result.Items[i].Unit < strings.ToLower(result.Items[j].Name)+result.Items[j].Unit
	})
	return result
}

func ChangeInventory(objectID, actorID, actorName string, mutation InventoryMutation) error {
	inventoryMutex.Lock()
	defer inventoryMutex.Unlock()
	if objectID == "" || actorID == "" {
		return ErrInventoryInvalid
	}
	// Work on a copy; publish only after the complete file has been persisted.
	next := inventoryData{
		Catalog: append([]models.InventoryName{}, inventory.Catalog...),
		Items:   append([]models.InventoryItem{}, inventory.Items...),
		History: append([]models.InventoryChange{}, inventory.History...),
	}
	index := -1
	for i, item := range next.Items {
		if item.ObjectID == objectID && item.ID == mutation.ItemID {
			index = i
			break
		}
	}
	if mutation.Action != "add" {
		if index < 0 {
			return ErrInventoryNotFound
		}
		if mutation.Action != "adjust" && (mutation.Version < 1 || next.Items[index].Version != mutation.Version) {
			return ErrInventoryConflict
		}
	}
	if mutation.Action == "add" || mutation.Action == "update" {
		mutation.Name = strings.Join(strings.Fields(mutation.Name), " ")
		mutation.Note = strings.TrimSpace(mutation.Note)
		if mutation.Name == "" || utf8.RuneCountInString(mutation.Name) > 120 || utf8.RuneCountInString(mutation.Note) > 500 || !validInventoryUnit(mutation.Unit) {
			return fmt.Errorf("%w: название до 120 символов, примечание до 500 символов и единица измерения обязательны", ErrInventoryInvalid)
		}
		material := ""
		if mutation.Material != nil {
			material = strings.Join(strings.Fields(*mutation.Material), " ")
		} else if index >= 0 {
			material = next.Items[index].Material
		}
		if utf8.RuneCountInString(material) > 60 {
			return fmt.Errorf("%w: материал до 60 символов", ErrInventoryInvalid)
		}
		catalogIndex := -1
		for i, name := range next.Catalog {
			if sameInventoryName(name.Name, name.Unit, mutation.Name, mutation.Unit) {
				mutation.Name = name.Name
				if mutation.Material == nil && index < 0 {
					material = name.Material
				}
				catalogIndex = i
				break
			}
		}
		material = InventoryMaterial(mutation.Name, material)
		for _, entry := range next.Catalog {
			canonical := InventoryMaterial(entry.Name, entry.Material)
			if strings.EqualFold(canonical, material) {
				material = canonical
				break
			}
		}
		if catalogIndex < 0 {
			next.Catalog = append(next.Catalog, models.InventoryName{Name: mutation.Name, Unit: mutation.Unit, Material: material})
		} else if mutation.Action == "update" && mutation.Material != nil {
			next.Catalog[catalogIndex].Material = material
		}
		mutation.Material = &material
		for i, item := range next.Items {
			if item.ObjectID == objectID && sameInventoryName(item.Name, item.Unit, mutation.Name, mutation.Unit) {
				if mutation.Action == "update" && i != index {
					return ErrInventoryDuplicate
				}
				if mutation.Action == "add" {
					index = i
				}
			}
		}
	}
	change := models.InventoryChange{ID: uuid.NewString(), ObjectID: objectID, Action: mutation.Action, ActorID: actorID, ActorName: actorName, At: time.Now().UTC().Format(time.RFC3339Nano)}
	item := models.InventoryItem{ID: uuid.NewString(), ObjectID: objectID}
	if index >= 0 {
		item = next.Items[index]
		before := item
		change.Before = &before
	}
	switch mutation.Action {
	case "add":
		if mutation.Quantity <= 0 || mutation.Quantity > MaxInventoryQuantity {
			return fmt.Errorf("%w: при добавлении количество должно быть больше нуля", ErrInventoryInvalid)
		}
		item.Name, item.Unit = mutation.Name, mutation.Unit
		item.Quantity += mutation.Quantity
		if index < 0 {
			item.Note = mutation.Note
			item.Material = *mutation.Material
		} else if mutation.Note != "" && mutation.Note != item.Note {
			return fmt.Errorf("%w: позиция уже есть; измените её примечание через редактирование", ErrInventoryInvalid)
		}
	case "update":
		item.Name, item.Unit, item.Note, item.Quantity = mutation.Name, mutation.Unit, mutation.Note, mutation.Quantity
		item.Material = *mutation.Material
	case "quantity":
		item.Quantity = mutation.Quantity
	case "adjust":
		if mutation.Delta != 1000 && mutation.Delta != -1000 {
			return ErrInventoryInvalid
		}
		item.Quantity += mutation.Delta
	case "delete":
	default:
		return ErrInventoryInvalid
	}
	if item.Quantity < 0 || item.Quantity > MaxInventoryQuantity {
		return fmt.Errorf("%w: остаток должен быть от 0 до 1 000 000", ErrInventoryInvalid)
	}
	if mutation.Action == "delete" {
		next.Items = append(next.Items[:index], next.Items[index+1:]...)
	} else {
		item.Version++
		item.UpdatedAt = change.At
		change.After = &item
		if index < 0 {
			next.Items = append(next.Items, item)
		} else {
			next.Items[index] = item
		}
	}
	next.History = append(next.History, change)
	if err := saveInventory(next); err != nil {
		return err
	}
	inventory = next
	return nil
}

func saveInventory(next inventoryData) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return writeData(inventoryFile, data)
}
