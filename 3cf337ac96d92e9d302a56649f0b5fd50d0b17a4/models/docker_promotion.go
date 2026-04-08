package models

import "time"

type DockerPromotion struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Repository    string    `json:"repository"`     // "bitswan-editor" or "gitops"
	StagingTag    string    `json:"staging_tag"`     // e.g. "2026-23140813591-git-c9cd8f5"
	StagingDigest string    `json:"staging_digest"`  // e.g. "sha256:ca1b419..."
	PromotedBy    string    `json:"promoted_by"`
	PromotedAt    time.Time `json:"promoted_at"`
}
