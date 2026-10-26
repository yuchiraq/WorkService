package models

// ConstructionContent is a snapshot: later changes to an object or its defaults
// must not silently change an already prepared document.
type ConstructionContent struct {
	Kind     string            `json:"kind"`
	Number   string            `json:"number"`
	Fields   map[string]string `json:"fields"`
	Rows     []ConstructionRow `json:"rows"`
	Archived bool              `json:"archived"`
}

type ConstructionRow struct {
	ID       string            `json:"id"`
	Section  int               `json:"section"`
	Fields   map[string]string `json:"fields"`
	Reviewed bool              `json:"reviewed"`
	Sources  []string          `json:"sources,omitempty"`
}

type ConstructionRevision struct {
	Version int64               `json:"version"`
	At      string              `json:"at"`
	ActorID string              `json:"actorId"`
	Actor   string              `json:"actor"`
	Reason  string              `json:"reason"`
	Content ConstructionContent `json:"content"`
}

type ConstructionDocument struct {
	ID        string                 `json:"id"`
	ObjectID  string                 `json:"objectId"`
	Version   int64                  `json:"version"`
	Content   ConstructionContent    `json:"content"`
	UpdatedAt string                 `json:"updatedAt"`
	UpdatedBy string                 `json:"updatedBy"`
	History   []ConstructionRevision `json:"history,omitempty"`
}
