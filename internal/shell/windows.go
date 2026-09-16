package shell

import (
	"fmt"
	"os/exec"
	"strings"
)

// Windows keeps right click menus in the registry, under the current user.
const (
	dirKey        = `HKCU\Software\Classes\Directory\shell\LynxShare`
	dirBackground = `HKCU\Software\Classes\Directory\Background\shell\LynxShare`
	driveKey      = `HKCU\Software\Classes\Drive\shell\LynxShare`
)

// installWindows writes the registry keys that put the entry in Explorer.
func installWindows(exe string) (string, error) {
	type entry struct {
		key      string
		argument string
	}
	entries := []entry{
		{dirKey, "%1"},
		{dirBackground, "%V"},
		{driveKey, "%1"},
	}
	for _, e := range entries {
		if err := regAdd(e.key, "", "REG_SZ", MenuTitle); err != nil {
			return "", err
		}
		if err := regAdd(e.key, "Icon", "REG_SZ", exe); err != nil {
			return "", err
		}
		command := fmt.Sprintf(`"%s" menu share "%s"`, exe, e.argument)
		if err := regAdd(e.key+`\command`, "", "REG_SZ", command); err != nil {
			return "", err
		}
	}
	return dirKey, nil
}

func uninstallWindows() error {
	var failed []string
	for _, key := range []string{dirKey, dirBackground, driveKey} {
		if err := regDelete(key); err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) == 3 {
		return fmt.Errorf("remove the registry keys: %s", strings.Join(failed, ", "))
	}
	return nil
}

func regAdd(key, name, kind, value string) error {
	args := []string{"add", key, "/f", "/t", kind, "/d", value}
	if name != "" {
		args = append(args, "/v", name)
	} else {
		args = append(args, "/ve")
	}
	out, err := exec.Command("reg", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("write %s: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

func regDelete(key string) error {
	return exec.Command("reg", "delete", key, "/f").Run()
}
