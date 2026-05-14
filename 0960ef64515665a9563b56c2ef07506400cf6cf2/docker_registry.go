package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	dockerHubAPI  = "https://hub.docker.com/v2"
	registryHost  = "https://registry-1.docker.io"
	authEndpoint  = "https://auth.docker.io/token"
	dockerService = "registry.docker.io"
)

// DockerTag represents a tag returned by the Docker Hub API.
type DockerTag struct {
	Name        string `json:"name"`
	FullSize    int64  `json:"full_size"`
	LastUpdated string `json:"last_updated"`
	Digest      string `json:"digest"`
}

type dockerHubTagsResponse struct {
	Count   int         `json:"count"`
	Next    string      `json:"next"`
	Results []DockerTag `json:"results"`
}

type tokenResponse struct {
	Token string `json:"token"`
}

// manifestRef is used to parse blob references from manifests.
type manifestSchema struct {
	MediaType string            `json:"mediaType"`
	Config    *descriptorRef    `json:"config,omitempty"`
	Layers    []descriptorRef   `json:"layers,omitempty"`
	Manifests []manifestPlatRef `json:"manifests,omitempty"`
}

type descriptorRef struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type manifestPlatRef struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// ListDockerTags fetches tags from Docker Hub for a given repository under bitswan/.
func ListDockerTags(repo string) ([]DockerTag, error) {
	var allTags []DockerTag
	pageURL := fmt.Sprintf("%s/repositories/bitswan/%s/tags/?page_size=100&ordering=-last_updated", dockerHubAPI, url.PathEscape(repo))

	for pageURL != "" {
		resp, err := http.Get(pageURL)
		if err != nil {
			return nil, fmt.Errorf("fetching tags: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("Docker Hub returned %d: %s", resp.StatusCode, string(body))
		}

		var result dockerHubTagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		allTags = append(allTags, result.Results...)
		pageURL = result.Next
	}
	return allTags, nil
}

// getRegistryToken obtains a bearer token from Docker's auth service.
func getRegistryToken(scopes ...string) (string, error) {
	u, _ := url.Parse(authEndpoint)
	q := u.Query()
	q.Set("service", dockerService)
	for _, s := range scopes {
		q.Add("scope", s)
	}
	u.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", u.String(), nil)

	user := os.Getenv("DOCKER_USER")
	pass := os.Getenv("DOCKER_PASSWORD")
	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth returned %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decoding token: %w", err)
	}
	return tok.Token, nil
}

// getManifest fetches a manifest from the registry, returning the raw bytes and content type.
func getManifest(repo, ref, token string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/v2/bitswan/%s/manifests/%s", registryHost, repo, ref)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.index.v1+json",
		"application/vnd.oci.image.manifest.v1+json",
	}, ", "))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("registry returned %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// mountBlob attempts to mount a blob from sourceRepo into targetRepo.
func mountBlob(targetRepo, digest, sourceRepo, token string) error {
	url := fmt.Sprintf("%s/v2/bitswan/%s/blobs/uploads/?mount=%s&from=bitswan/%s",
		registryHost, targetRepo, digest, sourceRepo)
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("mounting blob: %w", err)
	}
	defer resp.Body.Close()

	// 201 = mounted, 202 = upload started (blob didn't exist in source, shouldn't happen)
	if resp.StatusCode != 201 && resp.StatusCode != 202 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mount returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// putManifest pushes a manifest to the registry with the given tag.
func putManifest(repo, tag string, manifest []byte, contentType, token string) error {
	url := fmt.Sprintf("%s/v2/bitswan/%s/manifests/%s", registryHost, repo, tag)
	req, _ := http.NewRequest("PUT", url, strings.NewReader(string(manifest)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("putting manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("put manifest returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// collectBlobDigests extracts all blob digests referenced by a manifest.
// For manifest lists/indexes, it recursively fetches child manifests.
func collectBlobDigests(repo, token string, manifestBytes []byte) ([]string, error) {
	var m manifestSchema
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		return nil, err
	}

	var digests []string

	// Manifest list / OCI index: recurse into child manifests
	if len(m.Manifests) > 0 {
		for _, child := range m.Manifests {
			digests = append(digests, child.Digest)
			childData, _, err := getManifest(repo, child.Digest, token)
			if err != nil {
				return nil, fmt.Errorf("fetching child manifest %s: %w", child.Digest, err)
			}
			childDigests, err := collectBlobDigests(repo, token, childData)
			if err != nil {
				return nil, err
			}
			digests = append(digests, childDigests...)
		}
		return digests, nil
	}

	// Single manifest: config + layers
	if m.Config != nil {
		digests = append(digests, m.Config.Digest)
	}
	for _, layer := range m.Layers {
		digests = append(digests, layer.Digest)
	}
	return digests, nil
}

// PromoteImage copies an image from a staging repo to a production repo with the given tags.
// srcRepo: e.g. "bitswan-editor-staging", dstRepo: e.g. "bitswan-editor"
func PromoteImage(srcRepo, srcTag, dstRepo string, dstTags []string) error {
	// Get token with pull on source, push+pull on target
	srcScope := fmt.Sprintf("repository:bitswan/%s:pull", srcRepo)
	dstScope := fmt.Sprintf("repository:bitswan/%s:push,pull", dstRepo)
	token, err := getRegistryToken(srcScope, dstScope)
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}

	// Fetch the manifest from source
	manifest, contentType, err := getManifest(srcRepo, srcTag, token)
	if err != nil {
		return fmt.Errorf("getting source manifest: %w", err)
	}

	// Collect all blob digests referenced by the manifest
	blobDigests, err := collectBlobDigests(srcRepo, token, manifest)
	if err != nil {
		return fmt.Errorf("collecting blob digests: %w", err)
	}

	// Mount all blobs from source to target
	seen := make(map[string]bool)
	for _, digest := range blobDigests {
		if seen[digest] {
			continue
		}
		seen[digest] = true
		if err := mountBlob(dstRepo, digest, srcRepo, token); err != nil {
			return fmt.Errorf("mounting blob %s: %w", digest, err)
		}
	}

	// Push manifest with each destination tag
	for _, tag := range dstTags {
		if err := putManifest(dstRepo, tag, manifest, contentType, token); err != nil {
			return fmt.Errorf("pushing manifest as %s: %w", tag, err)
		}
	}

	return nil
}
