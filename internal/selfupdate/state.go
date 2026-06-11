package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type checkState struct {
	LastCheck time.Time `json:"last_check"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".work", "self-update.json"), nil
}

func loadCheckState() (checkState, error) {
	path, err := statePath()
	if err != nil {
		return checkState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return checkState{}, nil
		}
		return checkState{}, err
	}
	var st checkState
	if err := json.Unmarshal(data, &st); err != nil {
		return checkState{}, err
	}
	return st, nil
}

func saveCheckState(st checkState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func shouldCheckNow(interval time.Duration, force bool) (bool, error) {
	if force {
		return true, nil
	}
	st, err := loadCheckState()
	if err != nil {
		return true, err
	}
	if st.LastCheck.IsZero() {
		return true, nil
	}
	return time.Since(st.LastCheck) >= interval, nil
}

func markChecked() error {
	return saveCheckState(checkState{LastCheck: time.Now()})
}
