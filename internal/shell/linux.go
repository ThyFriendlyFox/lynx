package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// installLinux writes the two entries that cover most desktops: a Nautilus
// script for GNOME and a service menu for KDE.
func installLinux(exe string) (string, error) {
	var written []string

	script := strings.ReplaceAll(nautilusScript, "{{EXE}}", exe)
	if path, err := writeExecutable(nautilusPath(), script); err == nil {
		written = append(written, path)
	} else {
		return "", err
	}

	desktop := strings.ReplaceAll(kdeServiceMenu, "{{EXE}}", exe)
	if path, err := writeExecutable(kdePath(), desktop); err == nil {
		written = append(written, path)
	}

	return strings.Join(written, ", "), nil
}

func uninstallLinux() error {
	first := os.Remove(nautilusPath())
	second := os.Remove(kdePath())
	if os.IsNotExist(first) && os.IsNotExist(second) {
		return nil
	}
	if first != nil && !os.IsNotExist(first) {
		return first
	}
	if second != nil && !os.IsNotExist(second) {
		return second
	}
	return nil
}

func nautilusPath() string {
	return filepath.Join(dataHome(), "nautilus", "scripts", MenuTitle)
}

func kdePath() string {
	return filepath.Join(dataHome(), "kio", "servicemenus", "lynx-share.desktop")
}

func dataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "share")
}

func writeExecutable(path, content string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

const nautilusScript = `#!/bin/sh
# Lynx adds this script. Remove it with "lynx menu remove".
for folder in $NAUTILUS_SCRIPT_SELECTED_FILE_PATHS; do
  "{{EXE}}" menu share "$folder"
  exit 0
done
"{{EXE}}" menu share "$(pwd)"
`

const kdeServiceMenu = `[Desktop Entry]
Type=Service
ServiceTypes=KonqPopupMenu/Plugin
MimeType=inode/directory;
Actions=lynxShare
X-KDE-Priority=TopLevel

[Desktop Action lynxShare]
Name=Share with Lynx
Icon=folder-remote
Exec={{EXE}} menu share %f
`
