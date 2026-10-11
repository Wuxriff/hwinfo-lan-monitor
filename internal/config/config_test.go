package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil || c != Default() {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	want := Config{Port: 9000, LANAccess: false, HWiNFOPath: `C:\HWiNFO\HWiNFO64.exe`, StartInTray: true, Language: "ru", Window: &Point{X: -1200, Y: 40}}
	if err := Save(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got.Addr() != "127.0.0.1:9000" {
		t.Errorf("addr %q", got.Addr())
	}
}

func TestLoadFixesBadPort(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(p, []byte(`{"port": 0, "lanAccess": true}`), 0o644)
	c, _ := Load(p)
	if c.Port != 8086 || c.Addr() != ":8086" {
		t.Errorf("got %+v", c)
	}
}
