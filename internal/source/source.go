// Package source polls HWiNFO shared memory and publishes the latest state to any number of watchers.
package source

import (
	"bytes"
	"context"
	"errors"
	"hash/fnv"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
)

type Status string

const (
	StatusStarting    Status = "starting"
	StatusOK          Status = "ok"
	StatusUnavailable Status = "unavailable"
	StatusInactive    Status = "inactive"
	StatusError       Status = "error"
)

const (
	minInterval   = 100 * time.Millisecond
	maxInterval   = 5 * time.Second
	retryInterval = time.Second
)

// State is an immutable view of the latest poll. Rev changes when the set of
// Readings changes; Names when only Sensor names change (HWiNFO renames some
// sensors at runtime, e.g. PresentMon follows the foreground app); Seq on every sample.
type State struct {
	Status   Status
	Err      string
	Snapshot *shm.Snapshot
	Rev      uint64
	Names    uint64
	Seq      uint64
	Updated  time.Time
}

type Source struct {
	read func() ([]byte, error)

	mu      sync.Mutex
	state   State
	changed chan struct{}
}

func New(read func() ([]byte, error)) *Source {
	return &Source{
		read:    read,
		state:   State{Status: StatusStarting},
		changed: make(chan struct{}),
	}
}

// Current returns the latest state and a channel closed on the next change.
func (s *Source) Current() (State, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, s.changed
}

func (s *Source) publish(st State) {
	s.mu.Lock()
	s.state = st
	close(s.changed)
	s.changed = make(chan struct{})
	s.mu.Unlock()
}

// Run polls until ctx is done. It samples at half the HWiNFO polling period so
// a new HWiNFO sample is picked up with at most half a period of lag.
func (s *Source) Run(ctx context.Context) {
	var prev []byte
	for {
		wait := s.poll(&prev)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

func (s *Source) poll(prev *[]byte) time.Duration {
	cur, _ := s.Current()
	raw, err := s.read()
	if err != nil {
		*prev = nil
		st := State{Status: statusOf(err), Err: err.Error(), Rev: cur.Rev, Names: cur.Names, Seq: cur.Seq, Updated: time.Now()}
		if st.Status != cur.Status || st.Err != cur.Err {
			s.publish(st)
		}
		return retryInterval
	}
	if bytes.Equal(raw, *prev) && cur.Status == StatusOK {
		return intervalFor(cur.Snapshot)
	}
	snap, err := shm.Parse(raw)
	if err != nil {
		*prev = nil
		s.publish(State{Status: statusOf(err), Err: err.Error(), Rev: cur.Rev, Names: cur.Names, Seq: cur.Seq, Updated: time.Now()})
		return retryInterval
	}
	*prev = raw
	rev := cur.Rev
	if cur.Snapshot == nil || structureHash(snap) != structureHash(cur.Snapshot) {
		rev++
	}
	s.publish(State{Status: StatusOK, Snapshot: snap, Rev: rev, Names: namesHash(snap), Seq: cur.Seq + 1, Updated: time.Now()})
	return intervalFor(snap)
}

func statusOf(err error) Status {
	switch {
	case errors.Is(err, shm.ErrNotAvailable):
		return StatusUnavailable
	case errors.Is(err, shm.ErrInactive):
		return StatusInactive
	default:
		return StatusError
	}
}

func intervalFor(s *shm.Snapshot) time.Duration {
	d := s.PollingPeriod / 2
	return time.Duration(math.Max(float64(minInterval), math.Min(float64(maxInterval), float64(d))))
}

func structureHash(s *shm.Snapshot) uint64 {
	h := fnv.New64a()
	w := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	for _, se := range s.Sensors {
		w(strconv.FormatUint(uint64(se.ID), 16), strconv.FormatUint(uint64(se.Instance), 10))
		for _, r := range se.Readings {
			w(r.Label, r.Unit, r.Type.String(), strconv.FormatUint(uint64(r.ID), 10))
		}
	}
	return h.Sum64()
}

func namesHash(s *shm.Snapshot) uint64 {
	h := fnv.New64a()
	for _, se := range s.Sensors {
		h.Write([]byte(se.Name))
		h.Write([]byte{0})
	}
	return h.Sum64()
}
