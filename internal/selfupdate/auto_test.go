package selfupdate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldAutoUpdate(t *testing.T) {
	cfg := Config{Enabled: true}
	if ShouldAutoUpdate("dev", cfg) {
		t.Fatal("dev build should skip auto update")
	}
	if !ShouldAutoUpdate("v0.1.0", cfg) {
		t.Fatal("release build should allow auto update")
	}
	cfg.Enabled = false
	if ShouldAutoUpdate("v0.1.0", cfg) {
		t.Fatal("disabled config should skip")
	}
}

func TestShouldCheckNowRespectsInterval(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateDir := filepath.Join(home, ".work")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st := checkState{LastCheck: time.Now()}
	data, _ := json.Marshal(st)
	if err := os.WriteFile(filepath.Join(stateDir, "self-update.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	ok, err := shouldCheckNow(time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("should not check within interval")
	}

	ok, err = shouldCheckNow(time.Hour, true)
	if err != nil || !ok {
		t.Fatalf("force check failed: ok=%v err=%v", ok, err)
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	t.Setenv("WORK_AUTO_UPDATE", "false")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("WORK_AUTO_UPDATE=false should disable")
	}
}

func TestTryAutoSkipsWhenDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("WORK_AUTO_UPDATE", "false")

	res, err := TryAuto(context.Background(), AutoOptions{CurrentVersion: "v0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Checked || res.Updated {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".work")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(`
self_update:
  enabled: false
  check_interval: 12h
`)
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Fatal("expected disabled from yaml")
	}
	if cfg.CheckInterval != 12*time.Hour {
		t.Fatalf("got interval %v", cfg.CheckInterval)
	}
}
