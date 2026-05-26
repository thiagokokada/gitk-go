package gui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
)

type preferences struct {
	UIFont    []string `json:"ui_font,omitempty"`
	FixedFont []string `json:"fixed_font,omitempty"`
}

const (
	preferencesDirName  = "gitk-go"
	preferencesFileName = "preferences.json"
)

func (a *Controller) loadPreferences() {
	prefs, err := loadPreferencesFile()
	if err != nil {
		slog.Error("load preferences", slog.Any("error", err))
		return
	}
	a.prefs.uiFontSpec = slices.Clone(prefs.UIFont)
	a.prefs.fixedFontSpec = slices.Clone(prefs.FixedFont)
	a.applyStoredFontPreferences()
}

func (a *Controller) savePreferences(announce bool) {
	prefs := preferences{
		UIFont:    slices.Clone(a.prefs.uiFontSpec),
		FixedFont: slices.Clone(a.prefs.fixedFontSpec),
	}
	if err := savePreferencesFile(prefs); err != nil {
		a.ui.ShowMessage("Save Preferences", "error", fmt.Sprintf("Unable to save preferences:\n\n%v", err))
		return
	}
	if announce {
		a.setStatus("Preferences saved.")
	}
}

func loadPreferencesFile() (preferences, error) {
	path, err := preferencesPath()
	if err != nil {
		return preferences{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return preferences{}, nil
		}
		return preferences{}, err
	}
	prefs, err := parsePreferencesJSON(data)
	if err != nil {
		return preferences{}, err
	}
	slog.Debug("preferences loaded", slog.String("path", path))
	return prefs, nil
}

func savePreferencesFile(prefs preferences) error {
	path, err := preferencesPath()
	if err != nil {
		return err
	}
	data, err := encodePreferencesJSON(prefs)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(path, data); err != nil {
		return err
	}
	slog.Debug("preferences saved", slog.String("path", path))
	return nil
}

func preferencesPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, preferencesDirName, preferencesFileName), nil
}

func parsePreferencesJSON(data []byte) (preferences, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return preferences{}, nil
	}
	var prefs preferences
	if err := json.Unmarshal(trimmed, &prefs); err != nil {
		return preferences{}, err
	}
	return prefs, nil
}

func encodePreferencesJSON(prefs preferences) ([]byte, error) {
	return json.MarshalIndent(prefs, "", "  ")
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "preferences-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	removeTemp := func() {
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		removeTemp()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		removeTemp()
		return err
	}
	if err := tmp.Close(); err != nil {
		removeTemp()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		removeTemp()
		return err
	}
	return nil
}
