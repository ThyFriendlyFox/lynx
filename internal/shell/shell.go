// Package shell adds "Share with Lynx" to the right click menu of the file
// manager, and shows the share code to the person who clicks it.
package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/thyfriendlyfox/lynx/internal/daemon"
	"github.com/thyfriendlyfox/lynx/internal/folder"
)

// MenuTitle is the text a person sees in the right click menu.
const MenuTitle = "Share with Lynx"

// ErrUnsupported reports a system with no right click integration yet.
var ErrUnsupported = errors.New("lynx has no right click menu for this system yet")

// Install adds the menu entry and returns where it landed.
func Install() (string, error) {
	exe, err := exePath()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return installDarwin(exe)
	case "windows":
		return installWindows(exe)
	case "linux":
		return installLinux(exe)
	default:
		return "", ErrUnsupported
	}
}

// Uninstall removes the menu entry.
func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return os.RemoveAll(darwinServicePath())
	case "windows":
		return uninstallWindows()
	case "linux":
		return uninstallLinux()
	default:
		return ErrUnsupported
	}
}

// ShareFromMenu shares a folder and shows the code. The right click menu
// calls it.
func ShareFromMenu(dir string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		root = filepath.Dir(root)
	}
	fol, err := folder.Open(root)
	if errors.Is(err, folder.ErrNotShared) {
		fol, err = folder.Init(root)
	}
	if err != nil {
		return err
	}
	if _, err := daemon.Start(fol); err != nil && !errors.Is(err, daemon.ErrRunning) {
		return err
	}
	code := fol.ShareCode()
	copyToClipboard(code)
	message := fmt.Sprintf("Folder %s is shared.\n\nThe code is on your clipboard:\n\n%s\n\nSend it to a friend. They run: lynx join <code>",
		filepath.Base(fol.Root), code)
	showDialog(message)
	fmt.Println(code)
	return nil
}

func exePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// showDialog shows a message with the tool the system has.
func showDialog(message string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display dialog %s with title "Lynx" buttons {"OK"} default button 1 giving up after 300`, appleString(message))
		_ = exec.Command("osascript", "-e", script).Run()
	case "windows":
		ps := fmt.Sprintf(`Add-Type -AssemblyName PresentationFramework;[System.Windows.MessageBox]::Show(%s,'Lynx')`, powerShellString(message))
		_ = exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
	default:
		for _, try := range [][]string{
			{"zenity", "--info", "--title=Lynx", "--width=420", "--text=" + message},
			{"kdialog", "--title", "Lynx", "--msgbox", message},
			{"notify-send", "Lynx", message},
		} {
			if _, err := exec.LookPath(try[0]); err == nil {
				_ = exec.Command(try[0], try[1:]...).Run()
				return
			}
		}
	}
}

// copyToClipboard puts the code where a person can paste it.
func copyToClipboard(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		for _, name := range []string{"wl-copy", "xclip", "xsel"} {
			if _, err := exec.LookPath(name); err == nil {
				switch name {
				case "xclip":
					cmd = exec.Command(name, "-selection", "clipboard")
				case "xsel":
					cmd = exec.Command(name, "--clipboard", "--input")
				default:
					cmd = exec.Command(name)
				}
				break
			}
		}
	}
	if cmd == nil {
		return
	}
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

func appleString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// powerShellString quotes text for a PowerShell double quoted string, where
// the backtick is the escape character.
func powerShellString(s string) string {
	r := strings.NewReplacer("`", "``", `"`, "`\"", "$", "`$", "\n", "`n")
	return `"` + r.Replace(s) + `"`
}
