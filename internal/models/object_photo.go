package models

type ObjectPhoto struct {
	ID            string `json:"id"`
	ObjectID      string `json:"objectId"`
	Name          string `json:"name"`
	Caption       string `json:"caption"`
	CreatedAt     string `json:"createdAt"`
	CreatedByID   string `json:"createdById"`
	CreatedByName string `json:"createdByName"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	Version       int64  `json:"version"`
}
