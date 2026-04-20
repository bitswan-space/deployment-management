package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

func (app *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *App) handlePublicRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "BitSwan Deployment Manager API"})
}

// Allowlist of repositories the public Docker Hub tag proxy will serve.
// Restricts the endpoint from becoming a general-purpose Docker Hub proxy
// (which would attract abuse and burn our anonymous rate-limit budget).
// Covers the four images the AOC frontend needs for workspace updates.
var publicDockerHubRepos = map[string]bool{
	"gitops":                 true,
	"gitops-staging":         true,
	"bitswan-editor":         true,
	"bitswan-editor-staging": true,
}

// Cache the Docker Hub response per repo to absorb repeat reloads and
// limit how often we hit Docker Hub's anonymous endpoints. Stale is OK —
// tag lists churn slowly and the frontend only needs them accurate to the
// minute.
const dockerHubCacheTTL = 60 * time.Second

type dockerHubCacheEntry struct {
	tags      []DockerTag
	fetchedAt time.Time
}

var (
	dockerHubCache    = map[string]dockerHubCacheEntry{}
	dockerHubCacheMu  sync.Mutex
)

// handleListDockerTagsPublic proxies Docker Hub tag listings for an
// allowlisted bitswan/* repo. Public because the underlying data is itself
// public — but gated by the allowlist so this route can't be used to
// exfiltrate arbitrary Docker Hub metadata through our infrastructure.
func (app *App) handleListDockerTagsPublic(w http.ResponseWriter, r *http.Request) {
	repo := r.URL.Query().Get("repo")
	if repo == "" {
		writeError(w, http.StatusBadRequest, "repo query parameter is required")
		return
	}
	if !publicDockerHubRepos[repo] {
		writeError(w, http.StatusBadRequest, "repo not allowed")
		return
	}

	dockerHubCacheMu.Lock()
	if entry, ok := dockerHubCache[repo]; ok && time.Since(entry.fetchedAt) < dockerHubCacheTTL {
		dockerHubCacheMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"repo": repo, "tags": entry.tags, "cached": true})
		return
	}
	dockerHubCacheMu.Unlock()

	tags, err := ListDockerTags(repo)
	if err != nil {
		log.Printf("public: error listing tags for %s: %v", repo, err)
		writeError(w, http.StatusBadGateway, "failed to list tags from Docker Hub")
		return
	}

	dockerHubCacheMu.Lock()
	dockerHubCache[repo] = dockerHubCacheEntry{tags: tags, fetchedAt: time.Now()}
	dockerHubCacheMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"repo": repo, "tags": tags})
}
