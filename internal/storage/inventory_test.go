package storage

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func inventoryFixture(t *testing.T) {
	t.Helper()
	oldFile, oldData := inventoryFile, inventory
	t.Cleanup(func() { inventoryFile, inventory = oldFile, oldData })
	inventoryFile = filepath.Join(t.TempDir(), "inventory.json")
	if err := LoadInventory(); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryQuantities(t *testing.T) {
	for input, want := range map[string]int64{"0": 0, "1": 1000, "12,345": 12345, " 0.001 ": 1, "1000000": MaxInventoryQuantity, "01.2": 1200} {
		got, err := ParseInventoryQuantity(input)
		if err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", input, got, err, want)
		}
	}
	for _, input := range []string{"", "-1", "+1", "NaN", "Inf", "1e3", "0.0001", "1.", ".5", "1,2.3", "1 000", "1000000.001", "99999999999999999999999"} {
		if _, err := ParseInventoryQuantity(input); !errors.Is(err, ErrInventoryInvalid) {
			t.Errorf("accepted %q: %v", input, err)
		}
	}
}

func TestInventoryLifecycle(t *testing.T) {
	inventoryFixture(t)
	change := func(object string, mutation InventoryMutation) {
		t.Helper()
		if err := ChangeInventory(object, "actor", "Иван", mutation); err != nil {
			t.Fatal(err)
		}
	}
	change("one", InventoryMutation{Action: "add", Name: "  Кабель   силовой  ", Unit: "m", Quantity: 1250, Note: "Склад"})
	change("one", InventoryMutation{Action: "add", Name: "кабель силовой", Unit: "m", Quantity: 750})
	state := GetInventory("one")
	if len(state.Items) != 1 || len(state.Catalog) != 1 || state.Items[0].Quantity != 2000 || state.Items[0].Note != "Склад" {
		t.Fatalf("merged state: %+v", state)
	}
	item := state.Items[0]
	if item.Name != "Кабель силовой" || item.Version != 2 || len(state.History) != 2 || state.History[0].ActorID != "actor" {
		t.Fatalf("item/history: %+v", state)
	}
	change("two", InventoryMutation{Action: "add", Name: "КАБЕЛЬ СИЛОВОЙ", Unit: "m", Quantity: 1000})
	if len(GetInventory("two").Catalog) != 1 || len(GetInventory("two").History) != 1 {
		t.Fatal("catalog not reused or history leaked")
	}
	for _, action := range []string{"quantity", "delete", "adjust"} {
		if err := ChangeInventory("two", "actor", "", InventoryMutation{Action: action, ItemID: item.ID, Version: item.Version, Delta: 1000}); !errors.Is(err, ErrInventoryNotFound) {
			t.Fatalf("cross-object %s: %v", action, err)
		}
	}
	change("one", InventoryMutation{Action: "quantity", ItemID: item.ID, Version: item.Version, Quantity: 0})
	if err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "quantity", ItemID: item.ID, Version: item.Version, Quantity: 3000}); !errors.Is(err, ErrInventoryConflict) {
		t.Fatal("stale update accepted", err)
	}
	if err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "adjust", ItemID: item.ID, Delta: -1000}); !errors.Is(err, ErrInventoryInvalid) {
		t.Fatal("negative quantity accepted", err)
	}
	change("one", InventoryMutation{Action: "update", ItemID: item.ID, Version: 3, Name: "Лестница", Unit: "pcs", Note: "Шесть ступеней", Quantity: 1000})
	change("one", InventoryMutation{Action: "add", Name: "Аптечка", Unit: "set", Quantity: 1000})
	if err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "update", ItemID: item.ID, Version: 4, Name: "Аптечка", Unit: "set", Quantity: 1000}); !errors.Is(err, ErrInventoryDuplicate) {
		t.Fatal("duplicate update accepted", err)
	}
	state = GetInventory("one")
	if state.Items[0].Name != "Аптечка" {
		t.Fatal("items are not alphabetized")
	}
	state.Items[0].Name = "modified"
	state.History[0].After.Name = "modified"
	if GetInventory("one").Items[0].Name != "Аптечка" || GetInventory("one").History[0].After.Name != "Аптечка" {
		t.Fatal("snapshot exposes mutable storage")
	}
	change("one", InventoryMutation{Action: "delete", ItemID: item.ID, Version: 4})
	state = GetInventory("one")
	if len(state.Items) != 1 || len(state.Catalog) != 3 || state.History[0].Action != "delete" {
		t.Fatal("deletion removed the catalog/history", state)
	}
	if err := LoadInventory(); err != nil || !reflect.DeepEqual(state, GetInventory("one")) {
		t.Fatal("persisted state changed after reload", err)
	}
}

func TestInventoryConcurrentUpdatesAndRollback(t *testing.T) {
	inventoryFixture(t)
	if err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "add", Name: "Молоток", Unit: "pcs", Quantity: 1000}); err != nil {
		t.Fatal(err)
	}
	item := GetInventory("one").Items[0]
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "adjust", ItemID: item.ID, Delta: 1000}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	state := GetInventory("one")
	if state.Items[0].Quantity != 21000 || state.Items[0].Version != 21 {
		t.Fatal("concurrent increments lost", state.Items[0])
	}
	before := GetInventory("one")
	savedFile := inventoryFile
	inventoryFile = filepath.Join(inventoryFile, "cannot-create-inside-a-file.json")
	err := ChangeInventory("one", "actor", "", InventoryMutation{Action: "add", Name: "Не сохранено", Unit: "pcs", Quantity: 1000})
	if err == nil || !reflect.DeepEqual(before, GetInventory("one")) {
		t.Fatal("write failure changed memory", err)
	}
	inventoryFile = savedFile
	if err := os.WriteFile(inventoryFile, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadInventory(); err == nil || !reflect.DeepEqual(before, GetInventory("one")) {
		t.Fatal("corrupt file accepted or live data reset", err)
	}
}

func TestInventoryValidationDoesNotMutate(t *testing.T) {
	inventoryFixture(t)
	for _, mutation := range []InventoryMutation{
		{Action: "add", Name: "  ", Unit: "pcs", Quantity: 1},
		{Action: "add", Name: "Test", Unit: "invalid", Quantity: 1},
		{Action: "add", Name: "Test", Unit: "pcs", Quantity: 0},
		{Action: "add", Name: "Test", Unit: "pcs", Quantity: -1},
		{Action: "add", Name: "Test", Unit: "pcs", Quantity: MaxInventoryQuantity + 1},
	} {
		if err := ChangeInventory("one", "actor", "", mutation); !errors.Is(err, ErrInventoryInvalid) {
			t.Errorf("invalid mutation accepted: %+v, %v", mutation, err)
		}
		if len(GetInventory("one").Catalog) != 0 || len(GetInventory("one").History) != 0 {
			t.Fatal("validation mutated storage")
		}
	}
}
