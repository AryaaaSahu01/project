package objectstore

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
)

func TestLocalPrivateStore(t *testing.T) {
	s, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, key := range []string{"../outside", "assets/../outside", "C:/outside", "assets\\file", "assets//file"} {
		if err := s.Put(ctx, key, "image/png", 3, bytes.NewReader([]byte("abc"))); err == nil {
			t.Errorf("accepted unsafe key %q", key)
		}
	}
	key := "assets/workspace/asset"
	if err := s.Put(ctx, key, "audio/wav", 6, bytes.NewReader([]byte("abcdef"))); err != nil {
		t.Fatal(err)
	}
	body, err := s.ReadRange(ctx, key, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(data) != "cde" {
		t.Fatalf("bad range : %q %v", data, err)
	}
	if err := s.Put(ctx, "assets/workspace/incomplete", "audio/wav", 10, bytes.NewReader([]byte("short"))); err == nil {
		t.Fatal("accepted incomplete object")
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal("delete is not idempotent")
	}
	if b, err := s.ReadRange(ctx, key, 0, 1); !os.IsNotExist(err) {
		if b != nil {
			b.Close()
		}
		t.Fatalf("deleted file accessible: %v", err)
	}
}
