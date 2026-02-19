package updater

import (
	"WailsTest/config"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yeka/zip"
)

const (
	UPDATE_URL               = "https://files.ibello.cc/version.json"
	LAUNCHER_DOWNLOAD_URL    = "https://files.ibello.cc/LLauncher.zip"
	LIMBONIA_DOWNLOAD_URL    = "https://files.ibello.cc/Limbonia.zip"
	BOT_DOWNLOAD_URL         = "https://files.ibello.cc/BotQuixote.zip"
	CURRENT_LAUNCHER_VERSION = "4.0.0"
)

var ZIP_PASSWORD = ""

type UpdateResponse struct {
	LimboniaVersion string `json:"limbo_version"`
	LauncherVersion string `json:"launcher_version"`
	BotVersion      string `json:"bot_version"`
}

func CheckForUpdate() (needsUpdate bool, err error) {
	update := GetVersions()

	if update.LauncherVersion != CURRENT_LAUNCHER_VERSION {
		return true, nil
	}

	return false, nil
}

func GetVersions() UpdateResponse {
	res, err := http.DefaultClient.Get(UPDATE_URL)
	if err != nil {
		return UpdateResponse{}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return UpdateResponse{}
	}

	var update UpdateResponse
	err = json.Unmarshal(body, &update)
	if err != nil {
		return UpdateResponse{}
	}
	return update
}

func CheckForLimboniaVersion() (bool, error) {
	update := GetVersions()

	conf := config.Get()

	if conf == nil {
		return false, nil
	}

	if conf.CurrentLimboniaVersion != update.LimboniaVersion {
		return true, nil
	}
	return false, nil
}

// ExtractZipWithPassword extracts a password-protected zip file into destDir.
// Each file inside the archive is written to destDir preserving only its base name.
func ExtractZipWithPassword(zipPath, destDir, password string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	if err := os.MkdirAll(destDir, 0750); err != nil {
		return err
	}

	for _, f := range r.File {
		if f.IsEncrypted() {
			f.SetPassword(password)
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, filepath.Base(f.Name))
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
