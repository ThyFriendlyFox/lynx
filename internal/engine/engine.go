// Package engine keeps the folder on disk and the manifest in agreement, and
// applies what peers send.
package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/folder"
	"github.com/thyfriendlyfox/lynx/internal/ignore"
	"github.com/thyfriendlyfox/lynx/internal/index"
)

// MaxFileSize is the largest file this version moves between peers.
const MaxFileSize = 2 << 30 // 2 GiB

// ErrUnsafePath reports a path from a peer that points outside the folder.
var ErrUnsafePath = errors.New("path points outside the folder")

// Peer is the peer an entry came from. The identifier addresses it on the
// network. The name is what a person sees.
type Peer struct {
	ID   string
	Name string
}

// Fetcher gets the bytes of a file from a peer.
type Fetcher interface {
	Fetch(ctx context.Context, p Peer, e index.Entry, w io.Writer) error
}

// Event is one visible thing that happened to the folder.
type Event struct {
	Time time.Time `json:"time"`
	Kind string    `json:"kind"`
	Path string    `json:"path"`
	Peer string    `json:"peer,omitempty"`
}

// Engine owns the manifest of one folder.
type Engine struct {
	fol *folder.Folder
	ign *ignore.List
	man *index.Manifest
	log *slog.Logger

	mu      sync.Mutex
	fetcher Fetcher
	send    func([]index.Entry)

	evMu   sync.Mutex
	events []Event
}

// Open loads the manifest of a folder.
func Open(fol *folder.Folder, log *slog.Logger) (*Engine, error) {
	ign, err := ignore.Load(fol.Root)
	if err != nil {
		return nil, err
	}
	man, err := index.Load(fol.IndexPath())
	if err != nil {
		return nil, err
	}
	return &Engine{fol: fol, ign: ign, man: man, log: log}, nil
}

// SetFetcher sets how the engine gets file bytes from a peer.
func (e *Engine) SetFetcher(f Fetcher) { e.fetcher = f }

// SetSender sets how the engine tells peers about local changes.
func (e *Engine) SetSender(send func([]index.Entry)) { e.send = send }

// Folder returns the folder the engine works on.
func (e *Engine) Folder() *folder.Folder { return e.fol }

// Snapshot returns every entry, including tombstones, for a peer.
func (e *Engine) Snapshot() []index.Entry { return e.man.All() }

// Live returns the entries that are present on disk.
func (e *Engine) Live() []index.Entry { return e.man.Live() }

// ReloadIgnore reads .lynxignore again.
func (e *Engine) ReloadIgnore() {
	if ign, err := ignore.Load(e.fol.Root); err == nil {
		e.mu.Lock()
		e.ign = ign
		e.mu.Unlock()
	}
}

// Save writes the manifest to disk.
func (e *Engine) Save() error { return e.man.Save(e.fol.IndexPath()) }

// ScanLocal compares the folder on disk with the manifest, records every
// local change, and returns the changed entries.
func (e *Engine) ScanLocal() ([]index.Entry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.scanLocked()
}

func (e *Engine) scanLocked() ([]index.Entry, error) {
	found, err := index.Scan(e.fol.Root, e.ign)
	if err != nil {
		return nil, err
	}
	var changed []index.Entry
	now := index.NowMilli()
	me := e.fol.Config.PeerName

	for rel, st := range found {
		known, ok := e.man.Get(rel)
		if ok && !known.Deleted && known.Size == st.Size && known.ModTime == st.ModTime {
			continue
		}
		hash, err := index.HashFile(e.abs(rel))
		if err != nil {
			e.log.Warn("read file", "path", rel, "error", err)
			continue
		}
		if ok && !known.Deleted && known.Hash == hash {
			// Same bytes, new timestamp. Record the timestamp, tell nobody.
			known.Size = st.Size
			known.ModTime = st.ModTime
			known.Mode = st.Mode
			e.man.Put(known)
			continue
		}
		entry := index.Entry{
			Path:      rel,
			Size:      st.Size,
			ModTime:   st.ModTime,
			Mode:      st.Mode,
			Hash:      hash,
			Version:   e.man.NextVersion(rel),
			Origin:    me,
			UpdatedAt: now,
		}
		e.man.Put(entry)
		changed = append(changed, entry)
		kind := "change"
		if !ok || known.Deleted {
			kind = "add"
		}
		e.record(Event{Time: time.Now(), Kind: kind, Path: rel})
	}

	for _, known := range e.man.Live() {
		if _, still := found[known.Path]; still {
			continue
		}
		if e.ign.Match(known.Path, false) {
			continue
		}
		tomb := index.Entry{
			Path:      known.Path,
			Version:   known.Version + 1,
			Origin:    me,
			Deleted:   true,
			UpdatedAt: now,
		}
		e.man.Put(tomb)
		changed = append(changed, tomb)
		e.record(Event{Time: time.Now(), Kind: "delete", Path: known.Path})
	}

	if len(changed) > 0 {
		if err := e.Save(); err != nil {
			e.log.Warn("save index", "error", err)
		}
	}
	return changed, nil
}

// ScanAndSend records local changes and tells the peers about them.
func (e *Engine) ScanAndSend() (int, error) {
	changed, err := e.ScanLocal()
	if err != nil {
		return 0, err
	}
	if len(changed) > 0 && e.send != nil {
		e.send(changed)
	}
	return len(changed), nil
}

// ApplyManifest takes everything a peer knows and applies what is newer.
func (e *Engine) ApplyManifest(ctx context.Context, peer Peer, entries []index.Entry) error {
	if _, err := e.ScanLocal(); err != nil {
		return err
	}
	var firstErr error
	for _, remote := range entries {
		if err := e.ApplyRemote(ctx, peer, remote); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ApplyRemote applies one entry from a peer.
func (e *Engine) ApplyRemote(ctx context.Context, peer Peer, remote index.Entry) error {
	rel, err := safeRel(remote.Path)
	if err != nil {
		e.log.Warn("refused path from peer", "peer", peer.Name, "path", remote.Path)
		return err
	}
	remote.Path = rel

	e.mu.Lock()
	local, known := e.man.Get(rel)
	e.mu.Unlock()

	if known && !remote.Newer(local) {
		return nil
	}
	if remote.Deleted {
		return e.applyDelete(peer, remote, local, known)
	}
	if remote.Size > MaxFileSize {
		e.log.Warn("file is too large to sync", "path", rel, "size", remote.Size)
		return nil
	}
	return e.applyWrite(ctx, peer, remote, local, known)
}

func (e *Engine) applyWrite(ctx context.Context, peer Peer, remote, local index.Entry, known bool) error {
	if e.fetcher == nil {
		return errors.New("no peer connection to fetch from")
	}
	rel := remote.Path
	abs := e.abs(rel)

	// Fetch first, so that a failed transfer never touches the folder.
	tmp, err := os.CreateTemp(e.fol.StatePath(), "fetch-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := e.fetcher.Fetch(ctx, peer, remote, tmp); err != nil {
		tmp.Close()
		return fmt.Errorf("fetch %s: %w", rel, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	got, err := index.HashFile(tmpName)
	if err != nil {
		return err
	}
	if got != remote.Hash {
		return fmt.Errorf("fetch %s: the bytes do not match the hash", rel)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// The file may have changed while the transfer ran.
	current, hasCurrent := e.man.Get(rel)
	if hasCurrent && !remote.Newer(current) {
		return nil
	}
	if err := e.protect(rel, current, hasCurrent, remote, peer); err != nil {
		return err
	}

	mode := os.FileMode(remote.Mode).Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, abs); err != nil {
		return err
	}
	// Keep the peer's timestamp, so that the next scan sees no local change.
	when := time.UnixMilli(remote.ModTime)
	if err := os.Chtimes(abs, when, when); err != nil {
		e.log.Warn("set file time", "path", rel, "error", err)
	}
	stat, err := os.Stat(abs)
	if err != nil {
		return err
	}
	entry := remote
	entry.Size = stat.Size()
	entry.ModTime = stat.ModTime().UnixMilli()
	entry.Mode = uint32(stat.Mode().Perm())
	e.man.Put(entry)
	if err := e.Save(); err != nil {
		e.log.Warn("save index", "error", err)
	}
	kind := "receive"
	if !known || local.Deleted {
		kind = "receive-new"
	}
	e.record(Event{Time: time.Now(), Kind: kind, Path: rel, Peer: peer.Name})
	e.log.Info("received file", "path", rel, "peer", peer.Name, "bytes", entry.Size)
	return nil
}

func (e *Engine) applyDelete(peer Peer, remote, local index.Entry, known bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	rel := remote.Path
	abs := e.abs(rel)

	if known && !local.Deleted {
		if err := e.protect(rel, local, true, remote, peer); err != nil {
			return err
		}
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return err
	}
	removeEmptyParents(e.fol.Root, filepath.Dir(abs))
	e.man.Put(remote)
	if err := e.Save(); err != nil {
		e.log.Warn("save index", "error", err)
	}
	e.record(Event{Time: time.Now(), Kind: "delete-remote", Path: rel, Peer: peer.Name})
	e.log.Info("deleted file", "path", rel, "peer", peer.Name)
	return nil
}

// protect keeps local bytes that the incoming change would destroy. Bytes
// that two people wrote at the same version become a conflict copy next to
// the file. Every other overwritten or deleted file goes to .lynx/trash.
func (e *Engine) protect(rel string, local index.Entry, known bool, remote index.Entry, peer Peer) error {
	abs := e.abs(rel)
	onDisk, err := index.HashFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if onDisk == remote.Hash {
		return nil
	}

	concurrent := known && !local.Deleted && local.Version == remote.Version && local.Hash != remote.Hash
	unrecorded := !known || local.Deleted || onDisk != local.Hash
	if concurrent || unrecorded {
		name := index.ConflictName(rel, peer.Name, time.Now())
		if err := os.Rename(abs, e.abs(name)); err != nil {
			return err
		}
		e.record(Event{Time: time.Now(), Kind: "conflict", Path: name, Peer: peer.Name})
		e.log.Warn("kept a conflict copy", "path", name)
		return nil
	}
	return e.trash(rel, abs)
}

// trash moves a file into .lynx/trash. Nothing Lynx overwrites is lost.
func (e *Engine) trash(rel, abs string) error {
	dir := filepath.Join(e.fol.StatePath(), "trash")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	flat := strings.ReplaceAll(rel, "/", "_")
	dst := filepath.Join(dir, fmt.Sprintf("%d-%s", time.Now().UnixMilli(), flat))
	if err := os.Rename(abs, dst); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// Open returns the bytes of a file a peer asked for.
func (e *Engine) OpenFile(rel string, version uint64) (*os.File, index.Entry, error) {
	safe, err := safeRel(rel)
	if err != nil {
		return nil, index.Entry{}, err
	}
	entry, ok := e.man.Get(safe)
	if !ok || entry.Deleted {
		return nil, index.Entry{}, fmt.Errorf("no file at %s", safe)
	}
	if version != 0 && entry.Version != version {
		return nil, index.Entry{}, fmt.Errorf("version %d of %s is gone; this peer has version %d", version, safe, entry.Version)
	}
	f, err := os.Open(e.abs(safe))
	if err != nil {
		return nil, index.Entry{}, err
	}
	return f, entry, nil
}

// Events returns the last events, newest last.
func (e *Engine) Events(n int) []Event {
	e.evMu.Lock()
	defer e.evMu.Unlock()
	if n <= 0 || n > len(e.events) {
		n = len(e.events)
	}
	out := make([]Event, n)
	copy(out, e.events[len(e.events)-n:])
	return out
}

func (e *Engine) record(ev Event) {
	e.evMu.Lock()
	defer e.evMu.Unlock()
	e.events = append(e.events, ev)
	if len(e.events) > 500 {
		e.events = e.events[len(e.events)-500:]
	}
	e.appendLog(ev)
}

func (e *Engine) appendLog(ev Event) {
	line := fmt.Sprintf("%s\t%s\t%s\t%s\n", ev.Time.Format(time.RFC3339), ev.Kind, ev.Path, ev.Peer)
	f, err := os.OpenFile(e.fol.LogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func (e *Engine) abs(rel string) string {
	return filepath.Join(e.fol.Root, filepath.FromSlash(rel))
}

// safeRel rejects a path that leaves the folder or enters the state folder.
func safeRel(rel string) (string, error) {
	clean := filepath.ToSlash(filepath.Clean("/" + strings.TrimSpace(rel)))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", ErrUnsafePath
	}
	if strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", ErrUnsafePath
	}
	first := clean
	if i := strings.Index(clean, "/"); i >= 0 {
		first = clean[:i]
	}
	if first == folder.StateDir {
		return "", ErrUnsafePath
	}
	if filepath.IsAbs(rel) || strings.Contains(rel, "\\") {
		return "", ErrUnsafePath
	}
	return clean, nil
}

// removeEmptyParents deletes the directories a delete left behind.
func removeEmptyParents(root, dir string) {
	for dir != root && strings.HasPrefix(dir, root) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
