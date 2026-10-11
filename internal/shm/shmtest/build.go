// Package shmtest builds synthetic HWiNFO shared memory buffers for tests.
package shmtest

import (
	"encoding/binary"
	"math"
)

type Sensor struct {
	ID, Inst   uint32
	Orig, User string
	UserUTF    string
}

type Reading struct {
	Type, SensorIdx, ID  uint32
	Orig, User, Unit     string
	UserUTF, UnitUTF     string
	Value, Min, Max, Avg float64
}

// Build lays out a buffer like HWiNFO does, with element strides padded by extra bytes.
func Build(version uint32, extra uint32, sensors []Sensor, readings []Reading) []byte {
	sSize, rSize := uint32(0x108), uint32(0x13C)
	if version >= 2 {
		sSize, rSize = 0x188, 0x1CC
	}
	sSize += extra
	rSize += extra
	sOff := uint32(0x30)
	rOff := sOff + sSize*uint32(len(sensors))
	b := make([]byte, rOff+rSize*uint32(len(readings)))
	put := func(off, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
	put(0, 0x53695748)
	put(0x04, version)
	put(0x08, 1)
	binary.LittleEndian.PutUint64(b[0x0C:], 1_700_000_000)
	put(0x14, sOff)
	put(0x18, sSize)
	put(0x1C, uint32(len(sensors)))
	put(0x20, rOff)
	put(0x24, rSize)
	put(0x28, uint32(len(readings)))
	put(0x2C, 2000)
	for i, s := range sensors {
		e := b[sOff+uint32(i)*sSize:]
		binary.LittleEndian.PutUint32(e, s.ID)
		binary.LittleEndian.PutUint32(e[4:], s.Inst)
		copy(e[0x08:0x88], s.Orig)
		copy(e[0x88:0x108], s.User)
		if version >= 2 {
			copy(e[0x108:0x188], s.UserUTF)
		}
	}
	for i, r := range readings {
		e := b[rOff+uint32(i)*rSize:]
		binary.LittleEndian.PutUint32(e, r.Type)
		binary.LittleEndian.PutUint32(e[4:], r.SensorIdx)
		binary.LittleEndian.PutUint32(e[8:], r.ID)
		copy(e[0x0C:0x8C], r.Orig)
		copy(e[0x8C:0x10C], r.User)
		copy(e[0x10C:0x11C], r.Unit)
		for j, v := range []float64{r.Value, r.Min, r.Max, r.Avg} {
			binary.LittleEndian.PutUint64(e[0x11C+8*j:], math.Float64bits(v))
		}
		if version >= 2 {
			copy(e[0x13C:0x1BC], r.UserUTF)
			copy(e[0x1BC:0x1CC], r.UnitUTF)
		}
	}
	return b
}
