package models

// CloudItem is a private file or logical folder. Names are never filesystem paths.
type CloudItem struct {
	ID          string `json:"id"`
	OwnerID     string `json:"ownerId"`
	ParentID    string `json:"parentId"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	MIME        string `json:"mime,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
	Starred     bool   `json:"starred"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	DeletedAt   string `json:"deletedAt,omitempty"`
	TrashRootID string `json:"trashRootId,omitempty"`
	Version     int64  `json:"version"`
}
