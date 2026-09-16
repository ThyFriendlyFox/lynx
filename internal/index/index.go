// Package index holds what Lynx knows about the files in a folder.
package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/atomicfile"
	"github.com/thyfriendlyfox/lynx/internal/ignore"
)

// Entry is what Lynx knows about one path. A deleted path keeps its entry as
// a tombstone, because a peer that never saw the delete must still learn it.
type Entry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	ModTime   int64  `json:"mtime"`
	Hash      string `json:"hash"`
	Mode      uint32 `json:"mode"`
	Version   uint64 `json:"version"`
	Origin    string `json:"origin"`
	Deleted   bool   `json:"deleted,omitempty"`
	UpdatedAt int64  `json:"updated"`
}

// Newer reports whether e replaces other. Version decides. When two peers
// change the same path at the same version, the larger hash wins, so that
// both sides reach the same answer without talking.
func (e Entry) Newer(other Entry) bool {
	if e.Version != other.Version {
		return e.Version > other.Version
	}
	if e.Hash != other.Hash {
		return e.Hash > other.Hash
	}
	return false
}

// SameContent reports whether two entries describe the same bytes.
func (e Entry) SameContent(other Entry) bool {
	return e.Deleted == other.Deleted && e.Hash == other.Hash
}

// Manifest is the set of entries for a folder.
type Manifest struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// New returns an empty manifest.
func New() *Manifest {
	return &Manifest{entries: map[string]Entry{}}
}

// Get returns the entry for a path.
func (m *Manifest) Get(path string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[path]
	return e, ok
}

// Put stores an entry.
func (m *Manifest) Put(e Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[e.Path] = e
}

// All returns every entry, ordered by path.
func (m *Manifest) All() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, 0, len(m.entries))
	for _, e := range m.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Live returns the entries that are present on disk, ordered by path.
func (m *Manifest) Live() []Entry {
	out := m.All()
	live := out[:0]
	for _, e := range out {
		if !e.Deleted {
			live = append(live, e)
		}
	}
	return live
}

// Len returns the number of live entries.
func (m *Manifest) Len() int {
	return len(m.Live())
}

// NextVersion returns the version a new local change must carry for a path.
func (m *Manifest) NextVersion(path string) uint64 {
	if e, ok := m.Get(path); ok {
		return e.Version + 1
	}
	return 1
}

// Save writes the manifest to disk.
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(struct {
		Version int     `json:"version"`
		Entries []Entry `json:"entries"`
	}{Version: 1, Entries: m.All()}, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, data, 0o644)
}

// Load reads a manifest from disk. A missing file gives an empty manifest, so
// that a first start needs no setup step.
func Load(path string) (*Manifest, error) {
	m := New()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, err
	}
	var file struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		// A damaged index costs a rescan, never a crash.
		return m, nil
	}
	for _, e := range file.Entries {
		m.entries[e.Path] = e
	}
	return m, nil
}

// Stat is what a scan sees on disk before any hashing.
type Stat struct {
	Path    string
	Size    int64
	ModTime int64
	Mode    uint32
}

// Scan walks the folder and returns one Stat per file that is not ignored.
func Scan(root string, ign *ignore.List) (map[string]Stat, error) {
	found := map[string]Stat{}
	err := filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsPermission(err) || os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, relErr := filepath.Rel(root, abs)
		if relErr != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if ign.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			// Symlinks and devices do not cross the network in this version.
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		found[rel] = Stat{
			Path:    rel,
			Size:    info.Size(),
			ModTime: info.ModTime().UnixMilli(),
			Mode:    uint32(info.Mode().Perm()),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// HashFile returns the SHA-256 of a file as lower case hex.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// NowMilli returns the current time in milliseconds.
func NowMilli() int64 { return time.Now().UnixMilli() }

// ConflictName returns the name Lynx gives to the losing copy of a file that
// two people changed at the same time.
func ConflictName(rel, peer string, when time.Time) string {
	dir := ""
	base := rel
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		dir, base = rel[:i+1], rel[i+1:]
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if peer == "" {
		peer = "peer"
	}
	return dir + stem + " (conflict from " + peer + " " + when.Format("2006-01-02 1504") + ")" + ext
}
