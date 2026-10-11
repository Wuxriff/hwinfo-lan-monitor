// Package demo produces synthetic HWiNFO shared memory with generic hardware
// names and moving values, for screenshots and development without HWiNFO.
package demo

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm/shmtest"
)

const (
	typeTemp    = 1
	typeVolt    = 2
	typeFan     = 3
	typePower   = 5
	typeClock   = 6
	typeUsage   = 7
	typeOther   = 8
	cores       = 8
	pollingTick = 2 * time.Second
)

type reading struct {
	sensor uint32
	typ    uint32
	label  string
	unit   string
	gen    func(t float64) float64

	min, max, sum float64
	n             int
}

type Source struct {
	mu       sync.Mutex
	start    time.Time
	sensors  []shmtest.Sensor
	readings []*reading
	lastTick int64
	buf      []byte
}

func New() *Source {
	s := &Source{start: time.Now()}
	add := func(name string) uint32 {
		s.sensors = append(s.sensors, shmtest.Sensor{ID: 0xF0000100 + uint32(len(s.sensors)), UserUTF: name})
		return uint32(len(s.sensors) - 1)
	}
	r := func(sensor, typ uint32, label, unit string, gen func(float64) float64) {
		s.readings = append(s.readings, &reading{sensor: sensor, typ: typ, label: label, unit: unit, gen: gen})
	}
	wave := func(base, amp, period, phase float64) func(float64) float64 {
		return func(t float64) float64 {
			return base + amp*math.Sin(2*math.Pi*t/period+phase) + amp*0.25*(rand.Float64()*2-1)
		}
	}
	load := func(phase float64) func(float64) float64 {
		return func(t float64) float64 {
			v := 35 + 30*math.Sin(2*math.Pi*t/40+phase) + 15*rand.Float64()
			return math.Max(0, math.Min(100, v))
		}
	}
	step := func(values ...float64) func(float64) float64 {
		return func(float64) float64 { return values[rand.IntN(len(values))] }
	}
	constant := func(v float64) func(float64) float64 { return func(float64) float64 { return v } }

	sys := add("System: Demo PC")
	r(sys, typeOther, "Physical Memory Used", "MB", wave(14200, 900, 60, 0))
	r(sys, typeOther, "Physical Memory Available", "MB", wave(18500, 900, 60, math.Pi))
	r(sys, typeOther, "Physical Memory Load", "%", wave(43, 3, 60, 0))
	r(sys, typeOther, "Virtual Memory Load", "%", wave(38, 2, 80, 1))

	cpu := add("CPU [#0]: 8-Core Processor")
	for i := 0; i < cores; i++ {
		r(cpu, typeVolt, fmt.Sprintf("Core %d VID", i), "V", wave(1.25, 0.08, 17, float64(i)))
	}
	for i := 0; i < cores; i++ {
		r(cpu, typeClock, fmt.Sprintf("Core %d Clock", i), "MHz", step(3600, 4200, 4650, 4875, 5050))
	}
	r(cpu, typeClock, "Bus Clock", "MHz", constant(100))
	for i := 0; i < cores; i++ {
		for th := 0; th < 2; th++ {
			r(cpu, typeUsage, fmt.Sprintf("Core %d T%d Usage", i, th), "%", load(float64(i*2+th)))
		}
	}
	r(cpu, typeUsage, "Total CPU Usage", "%", load(0.5))
	r(cpu, typeTemp, "CPU (Tctl/Tdie)", "°C", wave(62, 9, 30, 0))
	r(cpu, typeTemp, "CPU Die (average)", "°C", wave(58, 8, 30, 0.3))
	r(cpu, typeVolt, "CPU Core Voltage", "V", wave(1.21, 0.05, 23, 0))
	r(cpu, typePower, "CPU Package Power", "W", wave(78, 28, 30, 0))
	for i := 0; i < cores; i++ {
		r(cpu, typePower, fmt.Sprintf("Core %d Power", i), "W", wave(6.5, 3, 25, float64(i)))
	}
	r(cpu, typeOther, "Thermal Throttling", "Yes/No", constant(0))

	gpu := add("GPU [#0]: Graphics Card")
	r(gpu, typeTemp, "GPU Temperature", "°C", wave(64, 6, 45, 0))
	r(gpu, typeTemp, "GPU Hot Spot Temperature", "°C", wave(76, 7, 45, 0.2))
	r(gpu, typeTemp, "GPU Memory Junction Temperature", "°C", wave(72, 5, 45, 0.4))
	r(gpu, typeVolt, "GPU Core Voltage", "V", wave(0.95, 0.07, 20, 0))
	r(gpu, typeFan, "GPU Fan1", "RPM", wave(1450, 180, 45, 0))
	r(gpu, typeFan, "GPU Fan2", "RPM", wave(1470, 180, 45, 0.1))
	r(gpu, typePower, "GPU Power", "W", wave(215, 60, 35, 0))
	r(gpu, typeClock, "GPU Clock", "MHz", wave(2580, 120, 25, 0))
	r(gpu, typeClock, "GPU Memory Clock", "MHz", constant(2500))
	r(gpu, typeUsage, "GPU Core Load", "%", load(2))
	r(gpu, typeUsage, "GPU Memory Usage", "%", wave(41, 4, 90, 0))
	r(gpu, typeOther, "GPU Memory Allocated", "MB", wave(6700, 400, 90, 0))
	r(gpu, typeOther, "Framerate", "FPS", wave(144, 18, 15, 0))

	mb := add("Motherboard (Super I/O)")
	r(mb, typeTemp, "Motherboard", "°C", wave(38, 1.5, 120, 0))
	r(mb, typeTemp, "Chipset", "°C", wave(52, 2, 120, 1))
	r(mb, typeVolt, "+12V", "V", wave(12.1, 0.03, 30, 0))
	r(mb, typeVolt, "+5V", "V", wave(5.04, 0.01, 30, 1))
	r(mb, typeVolt, "+3.3V", "V", wave(3.32, 0.01, 30, 2))
	r(mb, typeFan, "CPU Fan", "RPM", wave(1180, 160, 30, 0))
	r(mb, typeFan, "Chassis Fan 1", "RPM", wave(860, 40, 60, 0))
	r(mb, typeFan, "Chassis Fan 2", "RPM", wave(870, 40, 60, 1))

	drive := add("Drive: NVMe SSD 1TB")
	r(drive, typeTemp, "Drive Temperature", "°C", wave(41, 2, 90, 0))
	r(drive, typeOther, "Read Rate", "MB/s", step(0, 0, 0.4, 12.5, 210.8))
	r(drive, typeOther, "Write Rate", "MB/s", step(0, 0, 1.2, 35.6))
	r(drive, typeUsage, "Total Activity", "%", step(0, 0.3, 2.1, 8.4))

	net := add("Network: Ethernet")
	r(net, typeOther, "Current DL rate", "KB/s", step(0.8, 2.5, 140.2, 2310.6))
	r(net, typeOther, "Current UP rate", "KB/s", step(0.3, 1.1, 24.7))

	// Ten minutes of history so Min/Max/Average look like a running session.
	for t := -600.0; t < 0; t += 2 {
		s.sample(t)
	}
	return s
}

func (s *Source) sample(t float64) []float64 {
	out := make([]float64, len(s.readings))
	for i, r := range s.readings {
		v := math.Round(r.gen(t)*1000) / 1000
		if r.n == 0 || v < r.min {
			r.min = v
		}
		if r.n == 0 || v > r.max {
			r.max = v
		}
		r.sum += v
		r.n++
		out[i] = v
	}
	return out
}

// Read returns a new sample every HWiNFO-like polling tick and repeats it in between.
func (s *Source) Read() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.start)
	tick := int64(elapsed / pollingTick)
	if s.buf != nil && tick == s.lastTick {
		return s.buf, nil
	}
	s.lastTick = tick
	values := s.sample(elapsed.Seconds())
	out := make([]shmtest.Reading, len(s.readings))
	for i, r := range s.readings {
		out[i] = shmtest.Reading{Type: r.typ, SensorIdx: r.sensor, ID: uint32(i + 1), UserUTF: r.label, UnitUTF: r.unit,
			Value: values[i], Min: r.min, Max: r.max, Avg: r.sum / float64(r.n)}
	}
	s.buf = shmtest.Build(2, 0, s.sensors, out)
	return s.buf, nil
}
