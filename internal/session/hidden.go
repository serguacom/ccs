package session

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Hidden struct {
	path  string
	ids   map[string]bool
	dirty bool
}

// LoadHidden tolerates only a missing file: any other read error must abort,
// or Save would overwrite the list with just this run's toggles.
func LoadHidden(path string) (*Hidden, error) {
	h := &Hidden{path: path, ids: map[string]bool{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return h, nil
	}
	if err != nil {
		return nil, err
	}
	for line := range strings.Lines(string(data)) {
		if id := strings.TrimSpace(line); id != "" {
			h.ids[id] = true
		}
	}
	return h, nil
}

func (h *Hidden) Has(id string) bool { return h.ids[id] }

func (h *Hidden) Toggle(id string) {
	if h.ids[id] {
		delete(h.ids, id)
	} else {
		h.ids[id] = true
	}
	h.dirty = true
}

// Save overwrites the file with this run's snapshot: concurrent ccs instances are last-writer-wins,
// and a crash mid-write can truncate the list. Both are accepted for a hand-editable ID list.
func (h *Hidden) Save() error {
	if !h.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	var content string
	if len(h.ids) > 0 {
		content = strings.Join(slices.Sorted(maps.Keys(h.ids)), "\n") + "\n"
	}
	return os.WriteFile(h.path, []byte(content), 0o644)
}
