package main

import (
	"context"
	"log"
	"os"
	"time"

	"journalapp/internal/platform/objectstore"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store, err := objectstore.NewR2(ctx, objectstore.R2Config{
		AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		Bucket:          os.Getenv("R2_BUCKET"),
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := store.CheckBucket(ctx); err != nil {
		log.Fatal("R2 check failed: check bucket, endpoint, token scope and access key")
	}
	// Signature creation does not upload or reveal an object; do not log the URL.
	if _, _, err := store.PresignUpload(ctx, "dev-check/example.png", "image/png"); err != nil {
		log.Fatal("R2 presign failed")
	}
	log.Println("R2 OK | bucket accessible and short-lived upload signature generated")
}
