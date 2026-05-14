package models

import "time"

type AutomationRelease struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ReleaseID   int64     `json:"release_id" gorm:"uniqueIndex"`
	TagName     string    `json:"tag_name"`
	ReleaseName string    `json:"release_name"`
	Body        string    `json:"body"`
	Assets      string    `json:"assets"` // JSON array of {name, size, content_type}
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
}
