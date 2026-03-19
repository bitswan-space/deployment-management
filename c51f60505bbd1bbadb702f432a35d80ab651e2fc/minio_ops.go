package main

import (
	"context"
	"log"
	"net"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultBucket = "releases"

func mustInitMinio() *minio.Client {
	host := envOr("MINIO_HOST", "localhost")

	// Docker hostnames with "__" are invalid per HTTP RFC, causing MinIO server
	// to reject the Host header. Resolve to IP to avoid this.
	if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
		host = addrs[0]
	}

	endpoint := host + ":9000"
	accessKey := envOr("MINIO_ROOT_USER", "minioadmin")
	secretKey := envOr("MINIO_ROOT_PASSWORD", "minioadmin")

	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
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
