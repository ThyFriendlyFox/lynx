package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/folder"
)

// Start runs the daemon in the background and waits until it is up.
func Start(fol *folder.Folder) (State, error) {
	if st, ok := Running(fol); ok {
		return st, ErrRunning
	}
	exe, err := os.Executable()
	if err != nil {
		return State{}, err
	}
	logFile, err := os.OpenFile(fol.LogPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return State{}, err
	}
	defer logFile.Close()

	cmd := exec.Command(exe, "run", "--dir", fol.Root)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = fol.Root
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return State{}, err
	}
	if err := cmd.Process.Release(); err != nil {
		return State{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		if st, ok := Running(fol); ok {
			return st, nil
		}
		select {
		case <-ctx.Done():
			return State{}, fmt.Errorf("lynx did not start; see %s", fol.LogPath())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// Stop ends the daemon of a folder.
func Stop(fol *folder.Folder) error {
	st, ok := Running(fol)
	if !ok {
		return nil
	}
	if err := stop(st.PID); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, still := Running(fol); !still {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not stop", st.PID)
}
