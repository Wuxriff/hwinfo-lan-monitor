package hwinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDarkMode(t *testing.T) {
	cases := map[string]struct {
		ini  string
		dark bool
	}{
		"light":   {"[Settings]\r\nTheme=3\r\nDarkMode=0\r\n", false},
		"dark":    {"[Settings]\r\nDarkMode=1\r\n", true},
		"missing": {"[Settings]\r\nTheme=3\r\n", true},
	}
	for name, c := range cases {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "HWiNFO64.INI"), []byte(c.ini), 0o644)
		dark, ok := DarkMode(filepath.Join(dir, "HWiNFO64.exe"))
		if !ok || dark != c.dark {
			t.Errorf("%s: dark=%v ok=%v", name, dark, ok)
		}
	}
	if _, ok := DarkMode(filepath.Join(t.TempDir(), "HWiNFO64.exe")); ok {
		t.Error("no INI must report ok=false")
	}
}

func TestFindRunningLive(t *testing.T) {
	if !IsRunning() {
		t.Skip("HWiNFO not running")
	}
	t.Logf("HWiNFO path: %q", FindRunning())
}
