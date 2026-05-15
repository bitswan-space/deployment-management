package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultBucket = "releases"

func mustInitMinio() *minio.Client {
	host := envOr("MINIO_HOST", "localhost")
	accessKey := envOr("MINIO_ROOT_USER", "minioadmin")
	secretKey := envOr("MINIO_ROOT_PASSWORD", "minioadmin")

	// Docker hostnames with "__" are invalid per HTTP RFC, causing MinIO
	// server to reject the Host header. We use a resolved IP as the
	// endpoint but supply a custom dialer that re-resolves the original
	// hostname on every connection so we survive container restarts.
	displayHost := host
	if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
		displayHost = addrs[0]
	}

	endpoint := displayHost + ":9000"

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	// If the original hostname needs resolution (contains "__"), override
	// the dialer to resolve it dynamically instead of relying on the
	// (potentially stale) IP baked into the endpoint.
	if host != displayHost {
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// addr is "resolvedIP:9000"; replace with original hostname
			_, port, _ := net.SplitHostPort(addr)
			return (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext(ctx, network, net.JoinHostPort(host, port))
		}
	}

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    false,
		Transport: transport,
	})
	if err != nil {
		log.Fatalf("failed to create MinIO client: %v", err)
	}
	return mc
}

func ensureBucket(mc *minio.Client) {
	ctx := context.Background()
	exists, err := mc.BucketExists(ctx, defaultBucket)
	if err != nil {
		log.Fatalf("checking bucket: %v", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, defaultBucket, minio.MakeBucketOptions{}); err != nil {
			log.Fatalf("creating bucket: %v", err)
		}
		log.Printf("created bucket: %s", defaultBucket)
	}
}
