package group

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
)

// TestLiveDump prints the groups inferred from a local shmdump (go test -run Live -v).
func TestLiveDump(t *testing.T) {
	b, err := os.ReadFile("../../testdata/live.json")
	if err != nil {
		t.Skip("no testdata/live.json; create it with shmdump")
	}
	var s shm.Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	for _, se := range s.Sensors {
		labels := make([]string, len(se.Readings))
		units := make([]string, len(se.Readings))
		for i, r := range se.Readings {
			labels[i], units[i] = r.Label, r.Unit
		}
		for _, g := range Infer(labels, units) {
			t.Logf("%-50.50s | %-28s x%-3d first=%q", se.Name, g.Name, g.Count, labels[g.Start])
		}
	}
}
