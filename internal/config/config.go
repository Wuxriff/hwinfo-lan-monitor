// Package config stores Host app settings in config.json next to the executable.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port        int    `json:"port"`
	LANAccess   bool   `json:"lanAccess"`
	HWiNFOPath  string `json:"hwinfoPath"`
	StartInTray bool   `json:"startInTray"`
	Language    string `json:"language"`
}

func Default() Config {
	return Config{Port: 8086, LANAccess: true}
}

func (c Config) Addr() string {
	host := "127.0.0.1"
	if c.LANAccess {
		host = ""
	}
	return host + ":" + strconv.Itoa(c.Port)
}

// Path returns config.json beside the running executable.
func Path() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Default(), err
	}
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = Default().Port
	}
	return c, nil
}

func Save(path string, c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
