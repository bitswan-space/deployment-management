package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"backend/models"
)

// Valid repository base names (without -staging suffix).
var validRepos = map[string]bool{
	"bitswan-editor": true,
	"gitops":         true,
	"coding-agent":   true,
}

func (app *App) handleListDockerTags(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repo query parameter is required")
		return
	}

	tags, err := ListDockerTags(repo)
	if err != nil {
		log.Printf("error listing tags for %s: %v", repo, err)
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"repo": repo, "tags": tags})
}

func (app *App) handlePromoteImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Repository string `json:"repository"` // "bitswan-editor" or "gitops"
		Tag        string `json:"tag"`         // the staging tag to promote
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if !validRepos[req.Repository] {
		writeError(w, http.StatusBadRequest, "invalid repository: must be 'bitswan-editor', 'gitops', or 'coding-agent'")
		return
	}
	if req.Tag == "" {
		writeError(w, http.StatusBadRequest, "tag is required")
		return
	}

	srcRepo := req.Repository + "-staging"
	dstRepo := req.Repository

	// Find the staging tag's digest and all associated tags
	stagingTags, err := ListDockerTags(srcRepo)
	if err != nil {
		log.Printf("error listing staging tags: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list staging tags")
		return
	}

	// Find the selected tag and its digest
	var selectedDigest string
	for _, t := range stagingTags {
		if t.Name == req.Tag {
			selectedDigest = t.Digest
			break
		}
	}
	if selectedDigest == "" {
		writeError(w, http.StatusNotFound, fmt.Sprintf("tag %s not found in %s", req.Tag, srcRepo))
		return
	}

	// Find all tags that share the same digest (these are the tag variants for this image)
	var tagsToPromote []string
	for _, t := range stagingTags {
		if t.Digest == selectedDigest {
			tagsToPromote = append(tagsToPromote, t.Name)
		}
	}

	// Ensure "latest" is included
	hasLatest := false
	for _, t := range tagsToPromote {
		if t == "latest" {
			hasLatest = true
			break
		}
	}
	if !hasLatest {
		tagsToPromote = append(tagsToPromote, "latest")
	}

	// Perform the promotion
	if err := PromoteImage(srcRepo, req.Tag, dstRepo, tagsToPromote); err != nil {
		log.Printf("error promoting image: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to promote image: "+err.Error())
		return
	}

	// Record the promotion
	username := getUsername(r)
	promotion := models.DockerPromotion{
		Repository:    req.Repository,
		StagingTag:    req.Tag,
		StagingDigest: selectedDigest,
		PromotedBy:    username,
		PromotedAt:    time.Now(),
	}
	if err := app.db.Create(&promotion).Error; err != nil {
		log.Printf("error recording promotion: %v", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "promoted",
		"repository":    req.Repository,
		"staging_tag":   req.Tag,
		"promoted_tags": tagsToPromote,
		"promoted_by":   username,
	})
}

func (app *App) handleListPromotions(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")

	var promotions []models.DockerPromotion
	query := app.db.Order("promoted_at desc").Limit(50)
	if repo != "" {
		query = query.Where("repository = ?", repo)
	}
	if err := query.Find(&promotions).Error; err != nil {
		log.Printf("error listing promotions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list promotions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"promotions": promotions})
}

// handleListAllRepoTags returns tags for all 4 repos plus promotion status.
func (app *App) handleListAllRepoTags(w http.ResponseWriter, r *http.Request) {
	type repoInfo struct {
		Name string      `json:"name"`
		Tags []DockerTag `json:"tags"`
	}

	repos := []string{"bitswan-editor-staging", "gitops-staging", "coding-agent-staging", "bitswan-editor", "gitops", "coding-agent"}
	var results []repoInfo

	for _, repo := range repos {
		tags, err := ListDockerTags(repo)
		if err != nil {
			log.Printf("error listing tags for %s: %v", repo, err)
			tags = []DockerTag{}
		}
		results = append(results, repoInfo{Name: repo, Tags: tags})
	}

	// Get latest promotions to mark which staging images are in production
	var promotions []models.DockerPromotion
	app.db.Order("promoted_at desc").Limit(100).Find(&promotions)

	// Build a map of promoted digests per repo
	promotedDigests := make(map[string]models.DockerPromotion)
	for _, p := range promotions {
		key := p.Repository + ":" + p.StagingDigest
		if _, exists := promotedDigests[key]; !exists {
			promotedDigests[key] = p
		}
	}

	type promotionInfo struct {
		Digest     string `json:"digest"`
		PromotedBy string `json:"promoted_by"`
		PromotedAt string `json:"promoted_at"`
	}

	promotedMap := make(map[string]promotionInfo)
	for key, p := range promotedDigests {
		promotedMap[key] = promotionInfo{
			Digest:     p.StagingDigest,
			PromotedBy: p.PromotedBy,
			PromotedAt: p.PromotedAt.Format(time.RFC3339),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"repos":     results,
		"promoted":  promotedMap,
		"repo_pairs": []map[string]string{
			{"staging": "bitswan-editor-staging", "production": "bitswan-editor", "name": "BitSwan Editor"},
			{"staging": "gitops-staging", "production": "gitops", "name": "GitOps"},
			{"staging": "coding-agent-staging", "production": "coding-agent", "name": "Coding Agent"},
		},
	})
}

// handleDockerRepoStatus returns the current production tags alongside staging tags for a specific repo pair.
func (app *App) handleDockerRepoStatus(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" || !validRepos[repo] {
		writeError(w, http.StatusBadRequest, "repo must be 'bitswan-editor', 'gitops', or 'coding-agent'")
		return
	}

	stagingRepo := repo + "-staging"

	stagingTags, err := ListDockerTags(stagingRepo)
	if err != nil {
		log.Printf("error listing staging tags: %v", err)
		stagingTags = []DockerTag{}
	}

	prodTags, err := ListDockerTags(repo)
	if err != nil {
		log.Printf("error listing production tags: %v", err)
		prodTags = []DockerTag{}
	}

	// Group staging tags by digest
	type tagGroup struct {
		Digest      string      `json:"digest"`
		Tags        []DockerTag `json:"tags"`
		IsInProd    bool        `json:"is_in_prod"`
		DisplayName string      `json:"display_name"`
	}

	digestGroups := make(map[string]*tagGroup)
	var digestOrder []string
	for _, t := range stagingTags {
		if _, exists := digestGroups[t.Digest]; !exists {
			digestGroups[t.Digest] = &tagGroup{Digest: t.Digest}
			digestOrder = append(digestOrder, t.Digest)
		}
		digestGroups[t.Digest].Tags = append(digestGroups[t.Digest].Tags, t)
	}

	// Check which digests are in production
	prodDigests := make(map[string]bool)
	for _, t := range prodTags {
		prodDigests[t.Digest] = true
	}

	var groups []tagGroup
	for _, digest := range digestOrder {
		g := digestGroups[digest]
		g.IsInProd = prodDigests[digest]
		// Use the date-git tag as display name if available
		for _, t := range g.Tags {
			if strings.Contains(t.Name, "-git-") {
				g.DisplayName = t.Name
				break
			}
		}
		if g.DisplayName == "" && len(g.Tags) > 0 {
			g.DisplayName = g.Tags[0].Name
		}
		groups = append(groups, *g)
	}

	// Sort groups by newest first (most recently updated tag)
	sort.Slice(groups, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339Nano, groups[i].Tags[0].LastUpdated)
		tj, _ := time.Parse(time.RFC3339Nano, groups[j].Tags[0].LastUpdated)
		return ti.After(tj)
	})

	// Sort production tags by newest first
	sort.Slice(prodTags, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339Nano, prodTags[i].LastUpdated)
		tj, _ := time.Parse(time.RFC3339Nano, prodTags[j].LastUpdated)
		return ti.After(tj)
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"repository":      repo,
		"staging_repo":    stagingRepo,
		"staging_groups":  groups,
		"production_tags": prodTags,
	})
}
