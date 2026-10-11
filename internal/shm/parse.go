// Package shm decodes the HWiNFO shared memory sensor interface.
package shm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"time"
)

const (
	Signature  = 0x53695748
	headerSize = 0x2C
	stringLen  = 128
	unitLen    = 16

	sensorV1Size  = 0x108
	sensorV2Size  = sensorV1Size + stringLen
	readingV1Size = 0x13C
	readingV2Size = readingV1Size + stringLen + unitLen
)

var (
	ErrInactive  = errors.New("shared memory is inactive")
	ErrTruncated = errors.New("shared memory is truncated")
)

type ReadingType uint32

const (
	TypeNone ReadingType = iota
	TypeTemp
	TypeVolt
	TypeFan
	TypeCurrent
	TypePower
	TypeClock
	TypeUsage
	TypeOther
)

func (t ReadingType) String() string {
	names := [...]string{"none", "temp", "volt", "fan", "current", "power", "clock", "usage", "other"}
	if int(t) < len(names) {
		return names[t]
	}
	return "other"
}

func (t ReadingType) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

func (t *ReadingType) UnmarshalText(b []byte) error {
	for c := TypeNone; c <= TypeOther; c++ {
		if c.String() == string(b) {
			*t = c
			return nil
		}
	}
	*t = TypeOther
	return nil
}

type Snapshot struct {
	Version       uint32
	Revision      uint32
	PollTime      time.Time
	PollingPeriod time.Duration
	Sensors       []Sensor
}

type Sensor struct {
	ID       uint32
	Instance uint32
	NameOrig string
	Name     string
	Readings []Reading
}

type Reading struct {
	ID        uint32
	Type      ReadingType
	LabelOrig string
	Label     string
	Unit      string
	Value     float64
	Min       float64
	Max       float64
	Avg       float64
}

type layout struct {
	version, revision          uint32
	pollTime                   int64
	sensorOff, sensorSize      uint32
	sensorCount                uint32
	readingOff, readingSize    uint32
	readingCount, pollPeriodMs uint32
}

func parseHeader(b []byte) (layout, error) {
	if len(b) < headerSize {
		return layout{}, ErrTruncated
	}
	if binary.LittleEndian.Uint32(b) != Signature {
		return layout{}, ErrInactive
	}
	u := func(off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }
	l := layout{
		version:      u(0x04),
		revision:     u(0x08),
		pollTime:     int64(binary.LittleEndian.Uint64(b[0x0C:])),
		sensorOff:    u(0x14),
		sensorSize:   u(0x18),
		sensorCount:  u(0x1C),
		readingOff:   u(0x20),
		readingSize:  u(0x24),
		readingCount: u(0x28),
	}
	if l.revision >= 1 && len(b) >= headerSize+4 {
		l.pollPeriodMs = u(0x2C)
	}
	return l, nil
}

// Size returns how many bytes of the mapping the header says are in use.
func Size(b []byte) (int, error) {
	l, err := parseHeader(b)
	if err != nil {
		return 0, err
	}
	end := func(off, size, n uint32) uint64 { return uint64(off) + uint64(size)*uint64(n) }
	return int(max(uint64(headerSize), end(l.sensorOff, l.sensorSize, l.sensorCount), end(l.readingOff, l.readingSize, l.readingCount))), nil
}

// Parse decodes a full copy of the shared memory view.
func Parse(b []byte) (*Snapshot, error) {
	l, err := parseHeader(b)
	if err != nil {
		return nil, err
	}
	need, _ := Size(b)
	if len(b) < need {
		return nil, fmt.Errorf("%w: have %d bytes, header needs %d", ErrTruncated, len(b), need)
	}
	if l.sensorSize < sensorV1Size || (l.readingCount > 0 && l.readingSize < readingV1Size) {
		return nil, fmt.Errorf("unsupported element sizes %d/%d", l.sensorSize, l.readingSize)
	}
	utf := l.version >= 2

	s := &Snapshot{
		Version:       l.version,
		Revision:      l.revision,
		PollTime:      time.Unix(l.pollTime, 0),
		PollingPeriod: time.Duration(l.pollPeriodMs) * time.Millisecond,
		Sensors:       make([]Sensor, l.sensorCount),
	}
	for i := range s.Sensors {
		e := b[l.sensorOff+uint32(i)*l.sensorSize:][:l.sensorSize]
		name := cstr(e[0x88:0x108])
		if utf && l.sensorSize >= sensorV2Size {
			name = cstr(e[0x108:0x188])
		}
		s.Sensors[i] = Sensor{
			ID:       binary.LittleEndian.Uint32(e),
			Instance: binary.LittleEndian.Uint32(e[4:]),
			NameOrig: cstr(e[0x08:0x88]),
			Name:     name,
		}
	}
	for i := uint32(0); i < l.readingCount; i++ {
		e := b[l.readingOff+i*l.readingSize:][:l.readingSize]
		idx := binary.LittleEndian.Uint32(e[4:])
		if int(idx) >= len(s.Sensors) {
			continue
		}
		label, unit := cstr(e[0x8C:0x10C]), cstr(e[0x10C:0x11C])
		if utf && l.readingSize >= readingV2Size {
			label, unit = cstr(e[0x13C:0x1BC]), cstr(e[0x1BC:0x1CC])
		}
		f := func(off int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(e[off:])) }
		s.Sensors[idx].Readings = append(s.Sensors[idx].Readings, Reading{
			Type:      ReadingType(binary.LittleEndian.Uint32(e)),
			ID:        binary.LittleEndian.Uint32(e[8:]),
			LabelOrig: cstr(e[0x0C:0x8C]),
			Label:     label,
			Unit:      unit,
			Value:     f(0x11C),
			Min:       f(0x124),
			Max:       f(0x12C),
			Avg:       f(0x134),
		})
	}
	return s, nil
}

func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
