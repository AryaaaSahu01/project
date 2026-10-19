package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"journalapp/internal/platform/objectstore"
)

// A real private R2 upload/read-metadata/delete smoke check using a signed PUT.
// Only use this with a test bucket. This is not an unauthenticated API route.
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	store, err := objectstore.NewR2(ctx, objectstore.R2Config{
		AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		Bucket:          os.Getenv("R2_BUCKET"),
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
	})
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	key := "dev-check/" + hex.EncodeToString(id) + ".png"
	defer func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		if err := store.Delete(cleanCtx, key); err != nil {
			log.Print("WARNING: delete smoke-test object failed; remove it manually")
		}
	}()
	// A tiny 1x1 PNG. No personal user media is used in the smoke test.
	payload, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/euoAAAAASUVORK5CYII=",
	)
	if err != nil {
		return err
	}
	url, signedHeaders, err := store.PresignUpload(ctx, key, "image/png")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header = signedHeaders.Clone()
	req.Header.Set("Content-Type", "image/png")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload R2 smoke object: %w", err)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("signed PUT rejected: HTTP %d", response.StatusCode)
	}
	size, contentType, err := store.Head(ctx, key)
	if err != nil {
		return err
	}
	if size != int64(len(payload)) || contentType != "image/png" {
		return errors.New("R2 upload HEAD verification failed")
	}
	log.Printf("R2 smoke upload OK: image/png, %d bytes; cleanup queued", size)
	return nil
}
