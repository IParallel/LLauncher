package updater

import (
	"WailsTest/config"
	"encoding/json"
	"io"
	"net/http"
)

const (
	UPDATE_URL               = "https://files.ibello.site/static/version.json"
	LAUNCHER_DOWNLOAD_URL    = "https://files.ibello.site/static/LLauncher.exe"
	LIMBONIA_DOWNLOAD_URL    = "https://files.ibello.site/static/Limbus-Dumper.dll"
	BOT_DOWNLOAD_URL         = "https://files.ibello.site/static/BotQuixote.exe"
	CURRENT_LAUNCHER_VERSION = "3.0.0"
)

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
