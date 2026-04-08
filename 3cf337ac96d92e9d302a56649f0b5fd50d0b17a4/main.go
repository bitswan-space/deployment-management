package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode"

	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

// App holds shared dependencies.
type App struct {
	db   *gorm.DB
	mc   *minio.Client
	jwks *JWKSProvider
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

// capitalizeWords uppercases the first letter of each space-separated word.
func capitalizeWords(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	log.Printf("[ERROR] HTTP %d: %s", status, msg)
	writeJSON(w, status, map[string]string{"detail": msg})
}

// loggingMiddleware logs every incoming request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[REQUEST] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware wraps an http.Handler with permissive CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	log.SetOutput(os.Stdout)

	db := mustInitDB()
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	mc := mustInitMinio()
	ensureBucket(mc)

	issuerURL := mustEnv("KEYCLOAK_ISSUER_URL")
	jwks := NewJWKSProvider(issuerURL)

	app := &App{db: db, mc: mc, jwks: jwks}

	mux := http.NewServeMux()

	// Health (no auth)
	mux.HandleFunc("GET /health", app.handleHealth)

	// Public routes (no auth) - automation server downloads
	mux.HandleFunc("GET /public/", app.handlePublicRoot)
	mux.HandleFunc("GET /public/automation/releases", app.handleListPublishedReleases)
	mux.HandleFunc("GET /public/automation/latest", app.handleDownloadLatest)
	mux.HandleFunc("GET /public/automation/download/{tag}/{asset}", app.handleDownloadReleaseAsset)

	// Internal routes (auth required) - Docker image management
	mux.Handle("GET /internal/", app.requireAuth(http.HandlerFunc(app.handleInternalRoot)))
	mux.Handle("GET /internal/docker/tags", app.requireAuth(http.HandlerFunc(app.handleListDockerTags)))
	mux.Handle("GET /internal/docker/repos", app.requireAuth(http.HandlerFunc(app.handleListAllRepoTags)))
	mux.Handle("GET /internal/docker/status", app.requireAuth(http.HandlerFunc(app.handleDockerRepoStatus)))
	mux.Handle("POST /internal/docker/promote", app.requireAuth(http.HandlerFunc(app.handlePromoteImage)))
	mux.Handle("GET /internal/docker/promotions", app.requireAuth(http.HandlerFunc(app.handleListPromotions)))

	// Internal routes (auth required) - Automation server management
	mux.Handle("GET /internal/automation/releases", app.requireAuth(http.HandlerFunc(app.handleListGitHubReleases)))
	mux.Handle("POST /internal/automation/publish", app.requireAuth(http.HandlerFunc(app.handlePublishRelease)))
	mux.Handle("POST /internal/automation/unpublish", app.requireAuth(http.HandlerFunc(app.handleUnpublishRelease)))

	handler := loggingMiddleware(corsMiddleware(mux))

	log.Println("=== BACKEND BUILD 2026-04-08T18:00 WITH REQUEST LOGGING ===")
	log.Printf("stage=%s minio_host=%s", envOr("BITSWAN_AUTOMATION_STAGE", "unknown"), envOr("MINIO_HOST", "localhost"))
	log.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
