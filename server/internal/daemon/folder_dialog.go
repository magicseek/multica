package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var openNativeFolderDialogFn = openNativeFolderDialog

// openNativeFolderDialog opens an OS-native directory picker from the daemon
// process and returns the selected absolute path. It returns an empty path when
// the user cancels.
func openNativeFolderDialog() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return openFolderDialogMacOS()
	case "linux":
		return openFolderDialogLinux()
	case "windows":
		return openFolderDialogWindows()
	default:
		return "", fmt.Errorf("native folder picker is unavailable on %s", runtime.GOOS)
	}
}

func openFolderDialogMacOS() (string, error) {
	script := `tell application "System Events" to set frontApp to name of first process whose frontmost is true
tell application frontApp to activate
set chosenFolder to choose folder with prompt "Select repository folder"
return POSIX path of chosenFolder`

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func openFolderDialogLinux() (string, error) {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return "", fmt.Errorf("native folder picker is unavailable: DISPLAY/WAYLAND_DISPLAY is not set")
	}
	if path, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command(path, "--file-selection", "--directory", "--title=Select repository folder").Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
	if path, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command(path, "--getexistingdirectory", ".", "Select repository folder").Output()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
	return "", fmt.Errorf("native folder picker is unavailable: install zenity or kdialog")
}

func openFolderDialogWindows() (string, error) {
	ps := `Add-Type -AssemblyName System.Windows.Forms; ` +
		`$dialog = New-Object System.Windows.Forms.FolderBrowserDialog; ` +
		`$dialog.Description = 'Select repository folder'; ` +
		`if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::WriteLine($dialog.SelectedPath) }`
	out, err := exec.Command("powershell", "-NoProfile", "-STA", "-Command", ps).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
