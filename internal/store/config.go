package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/rztaylor/flashheart/internal/config"
)

// ConfigFile is the global settings file, relative to the root (CFG-1). It
// is the one declaration of its location.
const ConfigFile = ".flashheart/config.yaml"

// ReadConfig reads and validates the root's config.yaml through the root,
// so a symlink out of the root is refused (SEC-2). A missing file, or a
// root that does not exist yet, yields the defaults. Errors name the file.
func (s *Store) ReadConfig() (config.Config, error) {
	data, err := s.readConfigFile()
	if err != nil {
		return config.Config{}, fmt.Errorf("read %s: %w", s.configPath(), err)
	}
	if data == nil {
		return config.Defaults(), nil
	}
	settings, err := config.Parse(data)
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", s.configPath(), err)
	}
	return settings, nil
}

// UpdateConfig applies edit to config.yaml's content (nil when there is
// none) under the root lock (CFG-2).
func (s *Store) UpdateConfig(edit Edit) error {
	release, err := s.lock(".")
	if err != nil {
		return err
	}
	defer release()
	current, err := s.readConfigFile()
	if err != nil {
		return err
	}
	next, err := edit(current)
	if err != nil {
		return err
	}
	if bytes.Equal(next, current) {
		return nil
	}
	return s.WriteFileAtomic(ConfigFile, next)
}

// readConfigFile returns config.yaml, or nil when it or the root is missing.
func (s *Store) readConfigFile() ([]byte, error) {
	data, err := s.ReadFile(ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// configPath is config.yaml's path on disk, for messages.
func (s *Store) configPath() string {
	return filepath.Join(s.path, filepath.FromSlash(ConfigFile))
}
