package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/thyfriendlyfox/lynx/internal/folder"
	"github.com/thyfriendlyfox/lynx/internal/index"
)

// memFetcher serves file bytes from memory, in place of a peer.
type memFetcher struct{ files map[string]string }

func (m *memFetcher) Fetch(_ context.Context, _ Peer, e index.Entry, w io.Writer) error {
	body, ok := m.files[e.Path]
	if !ok {
		return os.ErrNotExist
	}
	_, err := io.WriteString(w, body)
	return err
}

func newEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	root := t.TempDir()
	fol, err := folder.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	eng, err := Open(fol, log)
	if err != nil {
		t.Fatal(err)
	}
	return eng, root
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsAddChangeDelete(t *testing.T) {
	eng, root := newEngine(t)

	write(t, root, "a.txt", "one")
	write(t, root, "deep/b.txt", "two")
	changed, err := eng.ScanLocal()
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("first scan found %d changes, want 2", len(changed))
	}

	if again, _ := eng.ScanLocal(); len(again) != 0 {
		t.Fatalf("second scan found %d changes, want 0", len(again))
	}

	write(t, root, "a.txt", "one changed")
	changed, _ = eng.ScanLocal()
	if len(changed) != 1 || changed[0].Path != "a.txt" || changed[0].Version != 2 {
		t.Fatalf("change scan gave %+v", changed)
	}

	if err := os.Remove(filepath.Join(root, "deep", "b.txt")); err != nil {
		t.Fatal(err)
	}
	changed, _ = eng.ScanLocal()
	if len(changed) != 1 || !changed[0].Deleted {
		t.Fatalf("delete scan gave %+v", changed)
	}
}

func TestScanSkipsStateFolder(t *testing.T) {
	eng, root := newEngine(t)
	write(t, root, ".lynx/secret.txt", "state")
	write(t, root, "keep.txt", "body")
	changed, err := eng.ScanLocal()
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0].Path != "keep.txt" {
		t.Fatalf("scan gave %+v, want only keep.txt", changed)
	}
}

func TestApplyRemoteWritesFile(t *testing.T) {
	eng, root := newEngine(t)
	eng.SetFetcher(&memFetcher{files: map[string]string{"from-peer.txt": "peer body"}})

	entry := index.Entry{
		Path: "from-peer.txt", Size: 9, Hash: hashOf("peer body"),
		Mode: 0o644, Version: 1, Origin: "friend", ModTime: index.NowMilli(),
	}
	if err := eng.ApplyRemote(context.Background(), Peer{ID: "id", Name: "friend"}, entry); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "from-peer.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "peer body" {
		t.Fatalf("file holds %q", got)
	}
	// The written file must not look like a local change afterwards.
	if changed, _ := eng.ScanLocal(); len(changed) != 0 {
		t.Fatalf("scan after apply found %+v", changed)
	}
}

func TestApplyRemoteKeepsConflictCopy(t *testing.T) {
	eng, root := newEngine(t)

	write(t, root, "shared.txt", "my own text")
	if _, err := eng.ScanLocal(); err != nil {
		t.Fatal(err)
	}
	// Two peers wrote version 1 of the same file. The larger hash wins, so
	// the test picks a body that wins.
	peerBody := bodyWithLargerHash(hashOf("my own text"))
	eng.SetFetcher(&memFetcher{files: map[string]string{"shared.txt": peerBody}})
	remote := index.Entry{
		Path: "shared.txt", Size: int64(len(peerBody)), Hash: hashOf(peerBody),
		Mode: 0o644, Version: 1, Origin: "friend", ModTime: index.NowMilli(),
	}
	if err := eng.ApplyRemote(context.Background(), Peer{ID: "id", Name: "friend"}, remote); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "shared.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != peerBody {
		t.Fatalf("shared.txt holds %q, want the winning copy", body)
	}
	names, _ := os.ReadDir(root)
	found := false
	for _, n := range names {
		if strings.Contains(n.Name(), "conflict from friend") {
			found = true
		}
	}
	if !found {
		t.Fatal("the losing copy was not kept")
	}
}

func TestApplyRemoteRefusesPathOutsideFolder(t *testing.T) {
	eng, _ := newEngine(t)
	eng.SetFetcher(&memFetcher{files: map[string]string{}})
	for _, bad := range []string{"../escape.txt", "/etc/passwd", ".lynx/config.json", "deep/../../out.txt"} {
		entry := index.Entry{Path: bad, Version: 1, Hash: "x", Origin: "attacker"}
		err := eng.ApplyRemote(context.Background(), Peer{ID: "id", Name: "attacker"}, entry)
		if err == nil {
			t.Fatalf("path %q was accepted", bad)
		}
	}
}

func TestOlderRemoteIsIgnored(t *testing.T) {
	eng, root := newEngine(t)
	eng.SetFetcher(&memFetcher{files: map[string]string{"a.txt": "old"}})
	write(t, root, "a.txt", "new")
	if _, err := eng.ScanLocal(); err != nil {
		t.Fatal(err)
	}
	local, _ := eng.man.Get("a.txt")
	remote := index.Entry{Path: "a.txt", Hash: hashOf("old"), Version: local.Version - 1, Origin: "friend"}
	if err := eng.ApplyRemote(context.Background(), Peer{ID: "id", Name: "friend"}, remote); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(body) != "new" {
		t.Fatalf("a.txt holds %q, want the local copy", body)
	}
}

// bodyWithLargerHash returns text whose hash sorts above the given hash.
func bodyWithLargerHash(than string) string {
	for i := 0; i < 10000; i++ {
		body := "peer text " + strconv.Itoa(i)
		if hashOf(body) > than {
			return body
		}
	}
	panic("no body with a larger hash")
}

func hashOf(body string) string {
	dir, err := os.MkdirTemp("", "hash")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "f")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		panic(err)
	}
	sum, err := index.HashFile(path)
	if err != nil {
		panic(err)
	}
	return sum
}
