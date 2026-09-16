// Package daemon runs one folder: it watches the files, talks to the peers,
// and writes the state that "lynx status" reads.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/atomicfile"
	"github.com/thyfriendlyfox/lynx/internal/engine"
	"github.com/thyfriendlyfox/lynx/internal/folder"
	"github.com/thyfriendlyfox/lynx/internal/ignore"
	"github.com/thyfriendlyfox/lynx/internal/p2p"
	"github.com/thyfriendlyfox/lynx/internal/watch"
)

// StateEvery is how often the daemon writes its state file.
const StateEvery = 5 * time.Second

// ErrRunning reports a folder that already has a daemon.
var ErrRunning = errors.New("lynx already runs in this folder")

// State is what a running daemon publishes about itself.
type State struct {
	PID       int            `json:"pid"`
	Root      string         `json:"root"`
	FolderID  string         `json:"folder_id"`
	PeerID    string         `json:"peer_id"`
	PeerName  string         `json:"peer_name"`
	Addrs     []string       `json:"addrs"`
	Started   time.Time      `json:"started"`
	Updated   time.Time      `json:"updated"`
	Files     int            `json:"files"`
	Peers     []p2p.PeerInfo `json:"peers"`
	LastEvent *engine.Event  `json:"last_event,omitempty"`
}

// Run keeps the folder in sync until the context ends.
func Run(ctx context.Context, fol *folder.Folder, log *slog.Logger) error {
	if st, ok := ReadState(fol); ok && alive(st.PID) && st.PID != os.Getpid() {
		return fmt.Errorf("%w (process %d)", ErrRunning, st.PID)
	}

	eng, err := engine.Open(fol, log)
	if err != nil {
		return err
	}
	node, err := p2p.New(fol, eng, log)
	if err != nil {
		return err
	}
	defer node.Close()

	if err := node.Start(ctx); err != nil {
		return err
	}
	log.Info("lynx started",
		"folder", fol.Root, "peer", fol.Config.PeerName, "id", node.ID(), "network", node.Topic())

	ign, err := ignore.Load(fol.Root)
	if err != nil {
		return err
	}
	w, err := watch.New(fol.Root, ign, log)
	if err != nil {
		return err
	}
	defer w.Close()
	go w.Run(ctx)

	if n, err := eng.ScanAndSend(); err != nil {
		log.Warn("first scan", "error", err)
	} else {
		log.Info("first scan done", "changes", n, "files", len(eng.Live()))
	}
	writeState(fol, eng, node)
	defer clearState(fol)

	ticker := time.NewTicker(StateEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("lynx stopped")
			return nil
		case <-w.Changed:
			eng.ReloadIgnore()
			if _, err := eng.ScanAndSend(); err != nil {
				log.Warn("scan", "error", err)
			}
			writeState(fol, eng, node)
		case <-ticker.C:
			writeState(fol, eng, node)
		}
	}
}

func writeState(fol *folder.Folder, eng *engine.Engine, node *p2p.Node) {
	st := State{
		PID:      os.Getpid(),
		Root:     fol.Root,
		FolderID: fol.ID(),
		PeerID:   node.ID(),
		PeerName: fol.Config.PeerName,
		Addrs:    node.Addrs(),
		Updated:  time.Now(),
		Files:    len(eng.Live()),
		Peers:    node.Peers(),
	}
	if old, ok := ReadState(fol); ok && old.PID == st.PID {
		st.Started = old.Started
	} else {
		st.Started = time.Now()
	}
	if events := eng.Events(1); len(events) == 1 {
		st.LastEvent = &events[0]
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = atomicfile.Write(fol.RunPath(), data, 0o644)
}

// ReadState returns the state a running daemon published.
func ReadState(fol *folder.Folder) (State, bool) {
	var st State
	data, err := os.ReadFile(fol.RunPath())
	if err != nil {
		return st, false
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, false
	}
	return st, true
}

// Running reports whether a daemon runs for this folder.
func Running(fol *folder.Folder) (State, bool) {
	st, ok := ReadState(fol)
	if !ok || !alive(st.PID) {
		return st, false
	}
	return st, true
}

func clearState(fol *folder.Folder) {
	_ = os.Remove(fol.RunPath())
}
