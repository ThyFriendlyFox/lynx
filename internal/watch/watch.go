// Package watch reports when something in the folder changes.
package watch

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/thyfriendlyfox/lynx/internal/ignore"
)

// Settle is how long the watcher waits after the last file event before it
// reports a change. An editor that writes a file in three steps then counts
// as one change.
const Settle = 400 * time.Millisecond

// FullScanEvery is the safety net. Some editors and some file systems hide
// events, so the watcher reports a change on this interval as well.
const FullScanEvery = 30 * time.Second

// Watcher reports folder changes on a channel.
type Watcher struct {
	root    string
	ign     *ignore.List
	log     *slog.Logger
	fsw     *fsnotify.Watcher
	Changed chan struct{}
}

// New starts watching root.
func New(root string, ign *ignore.List, log *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{root: root, ign: ign, log: log, fsw: fsw, Changed: make(chan struct{}, 1)}
	w.addTree(root)
	return w, nil
}

// Close stops the watcher.
func (w *Watcher) Close() error { return w.fsw.Close() }

// Run reports changes until the context ends.
func (w *Watcher) Run(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	pending := false
	full := time.NewTicker(FullScanEvery)
	defer full.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			if !w.interesting(ev) {
				continue
			}
			if ev.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					w.addTree(ev.Name)
				}
			}
			if !pending {
				pending = true
				timer.Reset(Settle)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(Settle)
			}
		case <-timer.C:
			pending = false
			w.signal()
		case <-full.C:
			w.signal()
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.log.Warn("watch error", "error", err)
		}
	}
}

func (w *Watcher) signal() {
	select {
	case w.Changed <- struct{}{}:
	default:
	}
}

func (w *Watcher) interesting(ev fsnotify.Event) bool {
	rel, err := filepath.Rel(w.root, ev.Name)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" {
		return false
	}
	info, statErr := os.Stat(ev.Name)
	isDir := statErr == nil && info.IsDir()
	return !w.ign.Match(rel, isDir)
}

// addTree watches a directory and every directory under it.
func (w *Watcher) addTree(dir string) {
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // an unreadable directory is skipped, not fatal
		}
		rel, relErr := filepath.Rel(w.root, path)
		if relErr == nil && rel != "." && w.ign.Match(filepath.ToSlash(rel), true) {
			return filepath.SkipDir
		}
		if err := w.fsw.Add(path); err != nil {
			w.log.Warn("watch directory", "path", path, "error", err)
		}
		return nil
	})
}
