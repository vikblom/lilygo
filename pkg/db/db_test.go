package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vikblom/lilygo/pkg/db"
)

func TestImageID(t *testing.T) {
	ctx := context.Background()

	fp := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := db.New(fp)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	want, err := db.AddImage(ctx, []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("add image: %s", err)
	}

	got, err := db.RandomImage(ctx)
	if err != nil {
		t.Fatalf("read image: %s", err)
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf(" (-want, +got):\n%s", d)
	}
}

func TestImageRead(t *testing.T) {
	ctx := context.Background()

	fp := filepath.Join(t.TempDir(), "db.sqlite")
	db, err := db.New(fp)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	id, err := db.AddImage(ctx, []byte{1, 2, 3})
	if err != nil {
		t.Fatalf("add image: %s", err)
	}

	// Just so its not the only possibility.
	for range 3 {
		_, err := db.AddImage(ctx, []byte{9, 9, 9})
		if err != nil {
			t.Fatalf("add image: %s", err)
		}
	}

	got, err := db.ReadImage(ctx, id)
	if err != nil {
		t.Fatalf("read image: %s", err)
	}

	want := []byte{1, 2, 3}
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf(" (-want, +got):\n%s", d)
	}
}
