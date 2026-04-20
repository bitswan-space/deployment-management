package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/models"

	"github.com/minio/minio-go/v7"
)

const releasesBucket = "releases"

// handleListGitHubReleases fetches releases from GitHub.
func (app *App) handleListGitHubReleases(w http.ResponseWriter, r *http.Request) {
	releases, err := FetchGitHubReleases()
	if err != nil {
		log.Printf("error fetching GitHub releases: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch releases from GitHub")
		return
	}

	// Get published releases from our DB
	var published []models.AutomationRelease
	app.db.Find(&published)
	publishedMap := make(map[int64]models.AutomationRelease)
	for _, p := range published {
		publishedMap[p.ReleaseID] = p
	}

	type releaseJSON struct {
		ID          int64         `json:"id"`
		TagName     string        `json:"tag_name"`
		Name        string        `json:"name"`
		Body        string        `json:"body"`
		Draft       bool          `json:"draft"`
		Prerelease  bool          `json:"prerelease"`
		CreatedAt   string        `json:"created_at"`
		PublishedAt string        `json:"published_at"`
		Assets      []GitHubAsset `json:"assets"`
		IsPublished bool          `json:"is_published"`
		PublishedBy string        `json:"published_by,omitempty"`
		LocalPubAt  string        `json:"local_published_at,omitempty"`
	}

	var result []releaseJSON
	for _, rel := range releases {
		entry := releaseJSON{
			ID:          rel.ID,
			TagName:     rel.TagName,
			Name:        rel.Name,
			Body:        rel.Body,
			Draft:       rel.Draft,
			Prerelease:  rel.Prerelease,
			CreatedAt:   rel.CreatedAt,
			PublishedAt: rel.PublishedAt,
			Assets:      rel.Assets,
		}
		if pub, ok := publishedMap[rel.ID]; ok {
			entry.IsPublished = true
			entry.PublishedBy = pub.PublishedBy
			entry.LocalPubAt = pub.PublishedAt.Format(time.RFC3339)
		}
		result = append(result, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{"releases": result})
}

// handlePublishRelease downloads release assets from GitHub and stores them in MinIO.
func (app *App) handlePublishRelease(w http.ResponseWriter, r *http.Request) {
	log.Printf("[PUBLISH] === publish handler entered ===")

	var req struct {
		ReleaseID int64 `json:"release_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	log.Printf("[PUBLISH] request to publish release_id=%d", req.ReleaseID)

	// Check if already published
	var existing models.AutomationRelease
	if err := app.db.Where("release_id = ?", req.ReleaseID).First(&existing).Error; err == nil {
		log.Printf("[PUBLISH] release_id=%d is already published, rejecting", req.ReleaseID)
		writeError(w, http.StatusConflict, "release is already published")
		return
	}

	// Fetch releases from GitHub to find this one
	log.Printf("[PUBLISH] fetching releases from GitHub...")
	releases, err := FetchGitHubReleases()
	if err != nil {
		log.Printf("[PUBLISH] error fetching GitHub releases: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch releases from GitHub")
		return
	}
	log.Printf("[PUBLISH] got %d releases from GitHub", len(releases))

	var release *GitHubRelease
	for _, rel := range releases {
		if rel.ID == req.ReleaseID {
			release = &rel
			break
		}
	}
	if release == nil {
		log.Printf("[PUBLISH] release_id=%d not found in GitHub releases", req.ReleaseID)
		writeError(w, http.StatusNotFound, "release not found on GitHub")
		return
	}
	log.Printf("[PUBLISH] found release %s (%s) with %d assets", release.TagName, release.Name, len(release.Assets))

	// Ensure releases bucket exists
	ctx := context.Background()
	log.Printf("[PUBLISH] checking MinIO bucket %q...", releasesBucket)
	exists, err := app.mc.BucketExists(ctx, releasesBucket)
	if err != nil {
		log.Printf("[PUBLISH] MinIO BucketExists error: %v", err)
		writeError(w, http.StatusInternalServerError, "storage error")
		return
	}
	log.Printf("[PUBLISH] bucket %q exists=%v", releasesBucket, exists)
	if !exists {
		log.Printf("[PUBLISH] creating bucket %q...", releasesBucket)
		if err := app.mc.MakeBucket(ctx, releasesBucket, minio.MakeBucketOptions{}); err != nil {
			log.Printf("[PUBLISH] MakeBucket error: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create storage bucket")
			return
		}
		log.Printf("[PUBLISH] bucket %q created", releasesBucket)
	}

	// Download and store each asset
	type assetInfo struct {
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		ContentType string `json:"content_type"`
	}
	var storedAssets []assetInfo

	for i, asset := range release.Assets {
		log.Printf("[PUBLISH] downloading asset %d/%d: %s from %s", i+1, len(release.Assets), asset.Name, asset.BrowserDownloadURL)
		data, contentType, err := DownloadGitHubAsset(asset.BrowserDownloadURL)
		if err != nil {
			log.Printf("[PUBLISH] download failed for %s: %v", asset.Name, err)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to download asset: %s", asset.Name))
			return
		}
		log.Printf("[PUBLISH] downloaded %s (%d bytes, %s)", asset.Name, len(data), contentType)

		key := fmt.Sprintf("%s/%s", release.TagName, asset.Name)
		log.Printf("[PUBLISH] storing asset in MinIO: bucket=%q key=%q", releasesBucket, key)
		_, err = app.mc.PutObject(ctx, releasesBucket, key, bytes.NewReader(data), int64(len(data)),
			minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			log.Printf("[PUBLISH] PutObject failed for %s: %v", asset.Name, err)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to store asset: %s", asset.Name))
			return
		}
		log.Printf("[PUBLISH] stored asset %s in MinIO", asset.Name)

		storedAssets = append(storedAssets, assetInfo{
			Name:        asset.Name,
			Size:        int64(len(data)),
			ContentType: contentType,
		})
	}

	// Serialize assets to JSON
	assetsJSON, _ := json.Marshal(storedAssets)

	username := getUsername(r)
	log.Printf("[PUBLISH] saving release record to DB (user=%s, tag=%s, %d assets)", username, release.TagName, len(storedAssets))
	record := models.AutomationRelease{
		ReleaseID:   release.ID,
		TagName:     release.TagName,
		ReleaseName: release.Name,
		Body:        release.Body,
		Assets:      string(assetsJSON),
		PublishedBy: username,
		PublishedAt: time.Now(),
	}
	if err := app.db.Create(&record).Error; err != nil {
		log.Printf("[PUBLISH] DB save failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to record published release")
		return
	}

	log.Printf("[PUBLISH] === release %s published successfully ===", release.TagName)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "published",
		"release_id":   release.ID,
		"tag":          release.TagName,
		"assets":       storedAssets,
		"published_by": username,
	})
}

// handleUnpublishRelease removes a published release.
func (app *App) handleUnpublishRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReleaseID int64 `json:"release_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	var release models.AutomationRelease
	if err := app.db.Where("release_id = ?", req.ReleaseID).First(&release).Error; err != nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}

	// Delete assets from MinIO
	var assets []struct {
		Name string `json:"name"`
	}
	json.Unmarshal([]byte(release.Assets), &assets)

	ctx := context.Background()
	for _, asset := range assets {
		key := fmt.Sprintf("%s/%s", release.TagName, asset.Name)
		app.mc.RemoveObject(ctx, releasesBucket, key, minio.RemoveObjectOptions{})
	}

	app.db.Delete(&release)
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpublished"})
}

// handleListPublishedReleases returns all published releases (public endpoint).
func (app *App) handleListPublishedReleases(w http.ResponseWriter, r *http.Request) {
	var releases []models.AutomationRelease
	app.db.Order("published_at desc").Find(&releases)

	type releaseJSON struct {
		TagName     string `json:"tag_name"`
		ReleaseName string `json:"release_name"`
		Body        string `json:"body"`
		Assets      []struct {
			Name        string `json:"name"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
		} `json:"assets"`
		PublishedAt string `json:"published_at"`
	}

	var result []releaseJSON
	for _, rel := range releases {
		entry := releaseJSON{
			TagName:     rel.TagName,
			ReleaseName: rel.ReleaseName,
			Body:        rel.Body,
			PublishedAt: rel.PublishedAt.Format(time.RFC3339),
		}
		json.Unmarshal([]byte(rel.Assets), &entry.Assets)
		result = append(result, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{"releases": result})
}

// handleDownloadReleaseAsset serves a published release asset from MinIO (public endpoint).
func (app *App) handleDownloadReleaseAsset(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	asset := r.PathValue("asset")
	if tag == "" || asset == "" {
		writeError(w, http.StatusBadRequest, "tag and asset are required")
		return
	}

	key := fmt.Sprintf("%s/%s", tag, asset)
	ctx := context.Background()
	obj, err := app.mc.GetObject(ctx, releasesBucket, key, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("error fetching asset %q from MinIO: %v", key, err)
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		log.Printf("error stat asset %q from MinIO: %v", key, err)
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", asset))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))

	buf := make([]byte, 32*1024)
	for {
		n, err := obj.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

// detectPlatform guesses os/arch from the User-Agent or query params.
// Query params ?os=linux&arch=amd64 take precedence.
func detectPlatform(r *http.Request) (string, string) {
	osParam := r.URL.Query().Get("os")
	archParam := r.URL.Query().Get("arch")

	if osParam == "" {
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		switch {
		case strings.Contains(ua, "darwin") || strings.Contains(ua, "mac"):
			osParam = "darwin"
		case strings.Contains(ua, "windows"):
			osParam = "windows"
		default:
			osParam = "linux"
		}
	}

	if archParam == "" {
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		if strings.Contains(ua, "arm64") || strings.Contains(ua, "aarch64") {
			archParam = "arm64"
		} else {
			archParam = "amd64"
		}
	}

	return osParam, archParam
}

// handleDownloadLatest picks the right platform asset from the latest release,
// extracts the binary from the tarball, and serves it directly.
func (app *App) handleDownloadLatest(w http.ResponseWriter, r *http.Request) {
	var release models.AutomationRelease
	if err := app.db.Order("published_at desc").First(&release).Error; err != nil {
		writeError(w, http.StatusNotFound, "no published releases")
		return
	}

	var assets []struct {
		Name        string `json:"name"`
		ContentType string `json:"content_type"`
	}
	json.Unmarshal([]byte(release.Assets), &assets)
	if len(assets) == 0 {
		writeError(w, http.StatusNotFound, "release has no assets")
		return
	}

	detectedOS, detectedArch := detectPlatform(r)
	suffix := fmt.Sprintf("%s-%s.tar.gz", detectedOS, detectedArch)

	// Find the matching asset
	var assetName string
	for _, a := range assets {
		if strings.HasSuffix(a.Name, suffix) {
			assetName = a.Name
			break
		}
	}
	if assetName == "" {
		// Fallback: try linux-amd64
		for _, a := range assets {
			if strings.HasSuffix(a.Name, "linux-amd64.tar.gz") {
				assetName = a.Name
				break
			}
		}
	}
	if assetName == "" {
		writeError(w, http.StatusNotFound, fmt.Sprintf("no asset found for %s/%s", detectedOS, detectedArch))
		return
	}

	key := fmt.Sprintf("%s/%s", release.TagName, assetName)
	ctx := context.Background()
	obj, err := app.mc.GetObject(ctx, releasesBucket, key, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("error fetching latest asset %q from MinIO: %v", key, err)
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	defer obj.Close()

	if _, err := obj.Stat(); err != nil {
		log.Printf("error stat latest asset %q from MinIO: %v", key, err)
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}

	// Extract the binary from the .tar.gz
	gzr, err := gzip.NewReader(obj)
	if err != nil {
		log.Printf("error decompressing asset %q: %v", key, err)
		writeError(w, http.StatusInternalServerError, "failed to decompress asset")
		return
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			log.Printf("error reading tar archive %q: %v", key, err)
			writeError(w, http.StatusInternalServerError, "no binary found in archive")
			return
		}
		if hdr.Typeflag == tar.TypeReg {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="bitswan"`)
			io.Copy(w, tr)
			return
		}
	}
}
