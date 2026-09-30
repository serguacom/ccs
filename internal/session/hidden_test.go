package session

import (
	"os"
	"path/filepath"
	"testing"
)

func mustLoad(t *testing.T, path string) *Hidden {
	t.Helper()
	h, err := LoadHidden(path)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestLoadHiddenMissingFileSavesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hidden")
	h := mustLoad(t, path)
	if h.Has("a") {
		t.Error("Has on empty set = true")
	}
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Save without Toggle created file: %v", err)
	}
}

func TestHiddenSaveWritesSortedAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hidden")
	h := mustLoad(t, path)
	h.Toggle("b")
	h.Toggle("a")
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\n" {
		t.Errorf("file = %q", data)
	}
	loaded := mustLoad(t, path)
	if !loaded.Has("a") || !loaded.Has("b") {
		t.Errorf("reloaded ids = %v", loaded.ids)
	}
}

func TestLoadHiddenUnreadableFileFails(t *testing.T) {
	if _, err := LoadHidden(t.TempDir()); err == nil {
		t.Error("LoadHidden on a directory returned no error")
	}
}

func TestHiddenDoubleToggleUnhidesButStillSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hidden")
	h := mustLoad(t, path)
	h.Toggle("a")
	h.Toggle("a")
	if h.Has("a") {
		t.Error("Has after double toggle = true")
	}
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 0 {
		t.Errorf("file = %q, err = %v", data, err)
	}
}
