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
	"strings"
)

const (
	Version   = "0.2.1"
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
func CheckForUpdates(sm *SessionManager) error {
	resp, err := http.Get("https://api.github.com/repos/MohamedElashri/txm/releases/latest")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %v", err)
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse release info: %v", err)
	}

	if release.TagName != Version {
		sm.logInfo(fmt.Sprintf("New version %s is available (current: %s)", release.TagName, Version))
		return nil
	}

	sm.logInfo("You are running the latest version")
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
	if err := os.Rename(newBinaryPath, inst.Path); err != nil {
		return fmt.Errorf("failed to replace binary: %v", err)
	}

	return os.Chmod(inst.Path, 0755)
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
