// Package cli is the lynx command line.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/thyfriendlyfox/lynx/internal/daemon"
	"github.com/thyfriendlyfox/lynx/internal/folder"
	"github.com/thyfriendlyfox/lynx/internal/sharecode"
	"github.com/thyfriendlyfox/lynx/internal/shell"
)

// Version is set at build time.
var Version = "dev"

const usage = `lynx keeps a folder the same on every computer that holds it.

Usage:
  lynx share [folder]        Share a folder and print the code to send
  lynx join <code> [folder]  Join a folder that a friend shared
  lynx status [folder]       Show the state of a folder
  lynx log [folder]          Show what happened in a folder
  lynx code [folder]         Print the share code again
  lynx start [folder]        Start syncing a folder
  lynx stop [folder]         Stop syncing a folder
  lynx run                   Sync in this window until you stop it
  lynx menu install          Add "Share with Lynx" to the right click menu
  lynx menu remove           Remove it again
  lynx version               Print the version

Run "lynx <command> -h" for the options of one command.
`

// Main runs the command line and returns the exit code.
func Main(args []string) int {
	if len(args) < 2 {
		fmt.Print(usage)
		return 2
	}
	cmd, rest := args[1], args[2:]
	var err error
	switch cmd {
	case "share":
		err = cmdShare(rest)
	case "join":
		err = cmdJoin(rest)
	case "status":
		err = cmdStatus(rest)
	case "log":
		err = cmdLog(rest)
	case "code":
		err = cmdCode(rest)
	case "start":
		err = cmdStart(rest)
	case "stop":
		err = cmdStop(rest)
	case "run":
		err = cmdRun(rest)
	case "menu":
		err = cmdMenu(rest)
	case "version", "--version", "-v":
		fmt.Printf("lynx %s\n", Version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "lynx: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "lynx: %s\n", err)
		return 1
	}
	return 0
}

func cmdShare(args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	dir := fs.String("dir", "", "the folder to share")
	codeOnly := fs.Bool("code-only", false, "print the share code and nothing else")
	noStart := fs.Bool("no-start", false, "do not start syncing")
	words, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	root, err := pickDir(*dir, words, 0)
	if err != nil {
		return err
	}

	fol, err := folder.Open(root)
	if errors.Is(err, folder.ErrNotShared) {
		fol, err = folder.Init(root)
	}
	if err != nil {
		return err
	}
	if !*noStart {
		if _, err := daemon.Start(fol); err != nil && !errors.Is(err, daemon.ErrRunning) {
			return err
		}
	}
	if *codeOnly {
		fmt.Println(fol.ShareCode())
		return nil
	}
	fmt.Printf("Folder %s is shared.\n\n", fol.Root)
	fmt.Printf("  %s\n\n", fol.ShareCode())
	fmt.Println("Send that code to a friend. On their computer they run:")
	fmt.Printf("  lynx join %s\n", fol.ShareCode())
	return nil
}

func cmdJoin(args []string) error {
	fs := flag.NewFlagSet("join", flag.ContinueOnError)
	dir := fs.String("dir", "", "the folder to sync into")
	noStart := fs.Bool("no-start", false, "do not start syncing")
	rest, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return errors.New("give the share code: lynx join <code> [folder]")
	}
	key, err := sharecode.Decode(rest[0])
	if err != nil {
		return err
	}
	root, err := pickDir(*dir, rest, 1)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	fol, err := folder.Join(root, key)
	if err != nil {
		return err
	}
	if !*noStart {
		if _, err := daemon.Start(fol); err != nil && !errors.Is(err, daemon.ErrRunning) {
			return err
		}
	}
	fmt.Printf("Folder %s is joined.\n", fol.Root)
	fmt.Println("Lynx now looks for the other copies of this folder.")
	fmt.Println("Run \"lynx status\" to see the peers and the files.")
	return nil
}

func cmdStart(args []string) error {
	fol, err := openFolderArgs("start", args)
	if err != nil {
		return err
	}
	st, err := daemon.Start(fol)
	if errors.Is(err, daemon.ErrRunning) {
		fmt.Printf("Lynx already runs in %s (process %d).\n", fol.Root, st.PID)
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("Lynx syncs %s (process %d).\n", fol.Root, st.PID)
	return nil
}

func cmdStop(args []string) error {
	fol, err := openFolderArgs("stop", args)
	if err != nil {
		return err
	}
	if _, running := daemon.Running(fol); !running {
		fmt.Printf("Lynx does not run in %s.\n", fol.Root)
		return nil
	}
	if err := daemon.Stop(fol); err != nil {
		return err
	}
	fmt.Printf("Lynx stopped in %s.\n", fol.Root)
	return nil
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	dir := fs.String("dir", "", "the folder to sync")
	verbose := fs.Bool("v", false, "print every network detail")
	words, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	root, err := pickDir(*dir, words, 0)
	if err != nil {
		return err
	}
	fol, err := folder.FindUp(root)
	if err != nil {
		return err
	}
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return daemon.Run(ctx, fol, log)
}

func cmdStatus(args []string) error {
	fol, err := openFolderArgs("status", args)
	if err != nil {
		return err
	}
	fmt.Printf("Folder:  %s\n", fol.Root)
	fmt.Printf("Name:    %s\n", fol.Config.Name)
	fmt.Printf("Machine: %s\n", fol.Config.PeerName)

	st, running := daemon.Running(fol)
	if !running {
		fmt.Println("Syncing: no. Run \"lynx start\" to start it.")
		return nil
	}
	fmt.Printf("Syncing: yes, since %s (process %d)\n", st.Started.Format(time.RFC822), st.PID)
	fmt.Printf("Files:   %d\n", st.Files)
	if len(st.Peers) == 0 {
		fmt.Println("Peers:   none yet. Lynx keeps looking.")
	} else {
		fmt.Printf("Peers:   %d\n", len(st.Peers))
		for _, p := range st.Peers {
			fmt.Printf("  - %s\n", p.Name)
		}
	}
	if st.LastEvent != nil {
		fmt.Printf("Last:    %s %s %s\n", st.LastEvent.Time.Format(time.RFC822), st.LastEvent.Kind, st.LastEvent.Path)
	}
	return nil
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ContinueOnError)
	dir := fs.String("dir", "", "the folder")
	lines := fs.Int("n", 20, "how many lines to show")
	words, err := parseArgs(fs, args)
	if err != nil {
		return err
	}
	root, err := pickDir(*dir, words, 0)
	if err != nil {
		return err
	}
	fol, err := folder.FindUp(root)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(fol.LogPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Nothing happened in this folder yet.")
			return nil
		}
		return err
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) > *lines {
		all = all[len(all)-*lines:]
	}
	fmt.Println(strings.Join(all, "\n"))
	return nil
}

func cmdCode(args []string) error {
	fol, err := openFolderArgs("code", args)
	if err != nil {
		return err
	}
	fmt.Println(fol.ShareCode())
	return nil
}

func cmdMenu(args []string) error {
	if len(args) == 0 {
		return errors.New("use \"lynx menu install\" or \"lynx menu remove\"")
	}
	switch args[0] {
	case "install":
		path, err := shell.Install()
		if err != nil {
			return err
		}
		fmt.Printf("The right click menu is installed: %s\n", path)
		fmt.Println("Right click a folder and choose \"Share with Lynx\".")
		return nil
	case "remove":
		if err := shell.Uninstall(); err != nil {
			return err
		}
		fmt.Println("The right click menu is removed.")
		return nil
	case "share":
		if len(args) < 2 {
			return errors.New("give the folder")
		}
		return shell.ShareFromMenu(args[1])
	default:
		return fmt.Errorf("unknown menu command %q", args[0])
	}
}

// parseArgs reads flags that come before or after the words. Go's flag
// package stops at the first word, and a person types "lynx join CODE --dir X"
// as often as the other order.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return words, nil
		}
		words = append(words, rest[0])
		args = rest[1:]
	}
}

// openFolderArgs reads the folder from a flag or the first argument.
func openFolderArgs(name string, args []string) (*folder.Folder, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	dir := fs.String("dir", "", "the folder")
	words, err := parseArgs(fs, args)
	if err != nil {
		return nil, err
	}
	root, err := pickDir(*dir, words, 0)
	if err != nil {
		return nil, err
	}
	return folder.FindUp(root)
}

// pickDir takes the folder from the flag, else from argument n, else the
// current folder.
func pickDir(flagDir string, args []string, n int) (string, error) {
	dir := flagDir
	if dir == "" && len(args) > n {
		dir = args[n]
	}
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return filepath.Abs(dir)
}
