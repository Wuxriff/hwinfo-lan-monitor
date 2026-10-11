package source

import (
	"testing"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm/shmtest"
)

type fakeReader struct {
	buf []byte
	err error
}

func (f *fakeReader) read() ([]byte, error) { return f.buf, f.err }

func buf(label string, value float64) []byte {
	return named("CPU", label, value)
}

func named(sensor, label string, value float64) []byte {
	return shmtest.Build(2, 0,
		[]shmtest.Sensor{{ID: 1, UserUTF: sensor}},
		[]shmtest.Reading{{ID: 1, UserUTF: label, UnitUTF: "°C", Value: value}})
}

func TestPollTracksStatusSeqAndRev(t *testing.T) {
	f := &fakeReader{err: shm.ErrNotAvailable}
	s := New(f.read)
	var prev []byte

	s.poll(&prev)
	st, changed := s.Current()
	if st.Status != StatusUnavailable || st.Seq != 0 {
		t.Fatalf("unavailable: %+v", st)
	}

	s.poll(&prev)
	select {
	case <-changed:
		t.Fatal("repeated error must not publish")
	default:
	}

	f.buf, f.err = buf("Tctl", 50), nil
	s.poll(&prev)
	st, changed = s.Current()
	if st.Status != StatusOK || st.Seq != 1 || st.Rev != 1 {
		t.Fatalf("first sample: %+v", st)
	}

	s.poll(&prev)
	select {
	case <-changed:
		t.Fatal("identical bytes must not publish")
	default:
	}

	f.buf = buf("Tctl", 51)
	s.poll(&prev)
	st, _ = s.Current()
	if st.Seq != 2 || st.Rev != 1 {
		t.Fatalf("new value keeps rev: %+v", st)
	}

	f.buf = buf("Tdie", 51)
	s.poll(&prev)
	st, _ = s.Current()
	if st.Seq != 3 || st.Rev != 2 {
		t.Fatalf("renamed reading bumps rev: %+v", st)
	}

	names := st.Names
	f.buf = named("PresentMon [dwm.exe]", "Tdie", 51)
	s.poll(&prev)
	st, _ = s.Current()
	if st.Rev != 2 || st.Names == names {
		t.Fatalf("renamed sensor changes names only: %+v", st)
	}
}
