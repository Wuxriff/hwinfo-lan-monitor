package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm/shmtest"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/source"
)

type event struct {
	name string
	data string
}

// readUntilValues collects events up to and including the first "values" event.
func readUntilValues(t *testing.T, sc *bufio.Scanner) []event {
	t.Helper()
	var out []event
	var cur event
	for (len(out) == 0 || out[len(out)-1].name != "values") && sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "" && cur.data != "":
			out = append(out, cur)
			cur = event{}
		}
	}
	if len(out) == 0 || out[len(out)-1].name != "values" {
		t.Fatalf("no values event in %+v: %v", out, sc.Err())
	}
	return out
}

func TestStreamSendsStatusStructureValues(t *testing.T) {
	buf := shmtest.Build(2, 0,
		[]shmtest.Sensor{{ID: 0xF0000300, UserUTF: "CPU [#0]"}},
		[]shmtest.Reading{
			{Type: 1, ID: 5, UserUTF: "Tctl", UnitUTF: "°C", Value: 61.23456, Min: 40, Max: 80, Avg: 55.5},
			{Type: 6, ID: 6, UserUTF: "Bus Clock", UnitUTF: "MHz", Value: 100},
		})
	src := source.New(func() ([]byte, error) { return buf, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Run(ctx)

	srv := httptest.NewServer(New(src, fstest.MapFS{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(nil, 1<<20)
	deadline := time.AfterFunc(5*time.Second, func() { resp.Body.Close() })
	defer deadline.Stop()

	evs := readUntilValues(t, sc)
	for len(evs) > 0 && evs[0].name == "status" && strings.Contains(evs[0].data, `"starting"`) {
		evs = evs[1:]
	}
	if evs[0].name != "status" || !strings.Contains(evs[0].data, `"ok"`) {
		t.Fatalf("status: %+v", evs[0])
	}
	var st wireStructure
	if evs[1].name != "structure" || json.Unmarshal([]byte(evs[1].data), &st) != nil {
		t.Fatalf("structure: %+v", evs[1])
	}
	if st.PeriodMs != 2000 || st.Sensors[0].Key != "f0000300.0" || st.Sensors[0].Readings[0].Key != "f0000300.0.5" || st.Sensors[0].Readings[1].Unit != "MHz" {
		t.Errorf("structure: %+v", st)
	}
	if evs[2].name != "values" || !strings.Contains(evs[2].data, `"v":[61.2346,40,80,55.5,100,0,0,0]`) {
		t.Errorf("values: %+v", evs[2])
	}
}

func TestStreamSendsNamesWhenOnlySensorRenamed(t *testing.T) {
	mk := func(name string) []byte {
		return shmtest.Build(2, 0, []shmtest.Sensor{{ID: 0xF0000FF0, UserUTF: name}}, []shmtest.Reading{{ID: 1, UserUTF: "FPS", Value: 60}})
	}
	var calls int
	src := source.New(func() ([]byte, error) {
		calls++
		if calls < 3 {
			return mk("PresentMon [game.exe]"), nil
		}
		return mk("PresentMon [dwm.exe]"), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Run(ctx)

	srv := httptest.NewServer(New(src, fstest.MapFS{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	deadline := time.AfterFunc(5*time.Second, func() { resp.Body.Close() })
	defer deadline.Stop()

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(nil, 1<<20)
	structures := 0
	for sc.Scan() {
		switch line := sc.Text(); {
		case line == "event: structure":
			structures++
		case strings.HasPrefix(line, "data: [") && strings.Contains(line, "dwm.exe"):
			if structures != 1 {
				t.Errorf("rename must not resend structure, got %d", structures)
			}
			return
		}
	}
	t.Fatal("no names event")
}
