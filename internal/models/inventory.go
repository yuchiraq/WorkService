package models

type InventoryName struct {
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Material string `json:"material,omitempty"`
}

type InventoryItem struct {
	ID        string `json:"id"`
	ObjectID  string `json:"object_id"`
	Name      string `json:"name"`
	Material  string `json:"material,omitempty"`
	Unit      string `json:"unit"`
	Quantity  int64  `json:"quantity"` // Thousandths of the selected unit.
	Note      string `json:"note"`
	Version   int64  `json:"version"`
	UpdatedAt string `json:"updated_at"`
}

type InventoryChange struct {
	ID        string         `json:"id"`
	ObjectID  string         `json:"object_id"`
	Action    string         `json:"action"`
	ActorID   string         `json:"actor_id"`
	ActorName string         `json:"actor_name"`
	At        string         `json:"at"`
	Before    *InventoryItem `json:"before,omitempty"`
	After     *InventoryItem `json:"after,omitempty"`
}
