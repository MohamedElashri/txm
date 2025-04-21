package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	Version   = "0.2.7"
	githubAPI = "https://api.github.com/repos/MohamedElashri/txm/releases/latest"
)

type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// InstallationType represents the type of installation
type InstallationType struct {
	Type      string
	Path      string
	IsSystem  bool
	ManDir    string
	ConfigDir string
	CacheDir  string
	LogDir    string
}

// GetInstallationType determines the installation type and paths
func GetInstallationType() (*InstallationType, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to determine executable path: %v", err)
	}

	homeDir := os.Getenv("HOME")
	var inst InstallationType

	if strings.HasPrefix(execPath, "/usr/local/bin") {
		inst = InstallationType{
			Type:      "system",
			Path:      execPath,
			IsSystem:  true,
			ManDir:    "/usr/local/share/man/man1",
			ConfigDir: filepath.Join(homeDir, ".txm"),
			CacheDir:  filepath.Join(homeDir, ".cache/txm"),
			LogDir:    filepath.Join(homeDir, ".local/share/txm"),
		}
	} else {
		userBinDir := filepath.Join(homeDir, ".local/bin")
		if strings.HasPrefix(execPath, userBinDir) {
			inst = InstallationType{
				Type:      "user",
				Path:      execPath,
				IsSystem:  false,
				ManDir:    filepath.Join(homeDir, ".local/share/man/man1"),
				ConfigDir: filepath.Join(homeDir, ".txm"),
				CacheDir:  filepath.Join(homeDir, ".cache/txm"),
				LogDir:    filepath.Join(homeDir, ".local/share/txm"),
			}
		} else {
			return nil, fmt.Errorf("unknown installation type")
		}
	}

	return &inst, nil
}

// Utility functions for version checking and updates

func compareVersions(current, latest string) (bool, error) {
	// Strip 'v' prefix if present
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	// Split versions into components
	currentParts := strings.Split(current, ".")
	latestParts := strings.Split(latest, ".")

	// Ensure both have 3 components
	if len(currentParts) != 3 || len(latestParts) != 3 {
		return false, fmt.Errorf("invalid version format")
	}

	// Compare major.minor.patch
	for i := 0; i < 3; i++ {
		curr, err1 := strconv.Atoi(currentParts[i])
		latest, err2 := strconv.Atoi(latestParts[i])
		if err1 != nil || err2 != nil {
			return false, fmt.Errorf("invalid version number")
		}

		if latest > curr {
			return true, nil // Update needed
		} else if curr > latest {
			return false, nil // Current version is newer
		}
	}

	return false, nil // Versions are equal
}

func CheckForUpdates(sm *SessionManager) error {
	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %v", err)
	}

	needsUpdate, err := compareVersions(Version, release.TagName)
	if err != nil {
		return fmt.Errorf("error comparing versions: %v", err)
	}

	if needsUpdate {
		sm.logInfo(fmt.Sprintf("New version %s is available (current: %s)", release.TagName, Version))
		return nil
	}

	sm.logInfo(fmt.Sprintf("You are running the latest version (%s)", Version))
	return nil
}

func UpdateBinary(sm *SessionManager) error {
	inst, err := GetInstallationType()
	if err != nil {
		return err
	}

	if inst.IsSystem && os.Getuid() != 0 {
		return fmt.Errorf("system-wide update requires root privileges")
	}

	// Get latest release info
	release, err := getLatestRelease()
	if err != nil {
		return err
	}

	// Compare versions
	needsUpdate, err := compareVersions(Version, release.TagName)
	if err != nil {
		return fmt.Errorf("error comparing versions: %v", err)
	}

	if !needsUpdate {
		sm.logInfo(fmt.Sprintf("Already running the latest version (%s)", Version))
		return nil
	}

	// Download and install update
	if err := downloadAndInstallUpdate(release, inst); err != nil {
		return err
	}

	sm.logInfo(fmt.Sprintf("Successfully updated to version %s", release.TagName))
	return nil
}

func UninstallTxm(sm *SessionManager) error {
	inst, err := GetInstallationType()
	if err != nil {
		return err
	}

	if inst.IsSystem && os.Getuid() != 0 {
		return fmt.Errorf("system-wide uninstall requires root privileges")
	}

	// Remove binary
	if err := os.Remove(inst.Path); err != nil {
		return fmt.Errorf("failed to remove binary: %v", err)
	}

	// Remove man page
	manPath := filepath.Join(inst.ManDir, "txm.1")
	os.Remove(manPath) // Ignore error if not exists

	// Remove configuration, cache, and logs
	os.RemoveAll(inst.ConfigDir)
	os.RemoveAll(inst.CacheDir)
	os.RemoveAll(inst.LogDir)

	// Remove shell completions
	homeDir, _ := os.UserHomeDir()
	os.Remove(filepath.Join(homeDir, ".local/share/bash-completion/completions/txm"))
	os.Remove(filepath.Join(homeDir, ".config/fish/completions/txm.fish"))
	os.Remove(filepath.Join(homeDir, ".zsh/completion/_txm"))

	sm.logInfo(fmt.Sprintf("Successfully uninstalled %s installation", inst.Type))
	return nil
}

// Helper functions
func getLatestRelease() (*Release, error) {
	resp, err := http.Get(githubAPI)
	if err != nil {
		return nil, fmt.Errorf("failed to get release info: %v", err)
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %v", err)
	}

	return &release, nil
}

func downloadAndInstallUpdate(release *Release, inst *InstallationType) error {
	osName := runtime.GOOS
	assetName := fmt.Sprintf("txm-%s.zip", strings.Title(osName))

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no compatible binary found for %s", osName)
	}

	tmpDir, err := os.MkdirTemp("", "txm-update")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := downloadAndExtract(downloadURL, tmpDir); err != nil {
		return err
	}

	newBinaryPath := filepath.Join(tmpDir, fmt.Sprintf("txm-%s", strings.Title(osName)))

	// Create a temporary file in the same directory as the target
	tmpTarget := inst.Path + ".tmp"

	// Copy the new binary to temporary location
	srcFile, err := os.Open(newBinaryPath)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(tmpTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temporary target file: %v", err)
	}
	defer dstFile.Close()

	// Copy the contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(tmpTarget) // Clean up on error
		return fmt.Errorf("failed to copy binary: %v", err)
	}

	// Ensure all data is written to disk
	if err := dstFile.Sync(); err != nil {
		os.Remove(tmpTarget)
		return fmt.Errorf("failed to sync file: %v", err)
	}

	// Close files before rename
	dstFile.Close()
	srcFile.Close()

	// Replace the old binary with the new one
	if err := os.Rename(tmpTarget, inst.Path); err != nil {
		os.Remove(tmpTarget)
		return fmt.Errorf("failed to replace binary: %v", err)
	}

	return nil
}

func downloadAndExtract(url, destDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download update: %v", err)
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp(destDir, "download-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save download: %v", err)
	}
	tmpFile.Close()

	return extractZip(tmpFile.Name(), destDir)
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %v", err)
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
