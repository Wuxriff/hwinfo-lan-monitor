package shm

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm/shmtest"
)

func TestParseV2UsesUTF8AndStrides(t *testing.T) {
	b := shmtest.Build(2, 24,
		[]shmtest.Sensor{
			{ID: 0xF0000100, Inst: 0, Orig: "CPU [#0]", User: "CPU ANSI", UserUTF: "Процессор"},
			{ID: 0xE0002000, Inst: 1, Orig: "GPU", User: "GPU", UserUTF: "GPU"},
		},
		[]shmtest.Reading{
			{Type: 6, SensorIdx: 0, ID: 7, Orig: "Core 0 Clock", User: "x", Unit: "MHz", UserUTF: "Ядро 0", UnitUTF: "МГц", Value: 4700, Min: 3675, Max: 4700, Avg: 3874.2},
			{Type: 1, SensorIdx: 1, ID: 1, Orig: "GPU Temperature", UserUTF: "GPU Temperature", UnitUTF: "°C", Value: 55.5},
			{Type: 1, SensorIdx: 0, ID: 9, Orig: "Tctl", UserUTF: "Tctl", UnitUTF: "°C", Value: 60},
		})

	s, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 2 || s.PollingPeriod != 2*time.Second || !s.PollTime.Equal(time.Unix(1_700_000_000, 0)) {
		t.Errorf("header: %+v", s)
	}
	if len(s.Sensors) != 2 {
		t.Fatalf("sensors: %d", len(s.Sensors))
	}
	cpu := s.Sensors[0]
	if cpu.Name != "Процессор" || cpu.NameOrig != "CPU [#0]" || cpu.ID != 0xF0000100 {
		t.Errorf("cpu: %+v", cpu)
	}
	if len(cpu.Readings) != 2 || cpu.Readings[1].Label != "Tctl" {
		t.Fatalf("cpu readings: %+v", cpu.Readings)
	}
	r := cpu.Readings[0]
	want := Reading{ID: 7, Type: TypeClock, LabelOrig: "Core 0 Clock", Label: "Ядро 0", Unit: "МГц", Value: 4700, Min: 3675, Max: 4700, Avg: 3874.2}
	if r != want {
		t.Errorf("reading:\n got %+v\nwant %+v", r, want)
	}
	if s.Sensors[1].Instance != 1 || s.Sensors[1].Readings[0].Unit != "°C" {
		t.Errorf("gpu: %+v", s.Sensors[1])
	}
}

func TestParseV1FallsBackToANSI(t *testing.T) {
	b := shmtest.Build(1, 0,
		[]shmtest.Sensor{{ID: 1, Orig: "Drive", User: "My Drive"}},
		[]shmtest.Reading{{Type: 1, ID: 2, Orig: "Temp", User: "Drive Temp", Unit: "C", Value: 40}})
	s, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if s.Sensors[0].Name != "My Drive" || s.Sensors[0].Readings[0].Label != "Drive Temp" || s.Sensors[0].Readings[0].Unit != "C" {
		t.Errorf("got %+v", s.Sensors[0])
	}
}

func TestParseRejectsInactiveAndTruncated(t *testing.T) {
	b := shmtest.Build(2, 0, []shmtest.Sensor{{ID: 1}}, []shmtest.Reading{{ID: 1}})
	if _, err := Parse(b[:len(b)-1]); !errors.Is(err, ErrTruncated) {
		t.Errorf("truncated: %v", err)
	}
	binary.LittleEndian.PutUint32(b, 0xDEADBEEF)
	if _, err := Parse(b); !errors.Is(err, ErrInactive) {
		t.Errorf("inactive: %v", err)
	}
}

func TestParseSkipsReadingWithBadSensorIndex(t *testing.T) {
	b := shmtest.Build(2, 0, []shmtest.Sensor{{ID: 1}}, []shmtest.Reading{{ID: 1, SensorIdx: 5}})
	s, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Sensors[0].Readings) != 0 {
		t.Errorf("unexpected readings: %+v", s.Sensors[0].Readings)
	}
}
