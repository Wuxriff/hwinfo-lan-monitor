// Package server serves the Dashboard and streams HWiNFO data to it over SSE.
package server

import (
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/source"
)

const (
	keepAlive    = 15 * time.Second
	writeTimeout = 10 * time.Second
	maxRefresh   = 60 * time.Second
)

type Server struct {
	src *source.Source
	mux *http.ServeMux

	mu        sync.Mutex
	structRev uint64
	structMsg []byte
	valuesSeq uint64
	valuesMsg []byte
}

func New(src *source.Source, static fs.FS) *Server {
	s := &Server{src: src, mux: http.NewServeMux()}
	s.mux.Handle("GET /", http.FileServerFS(static))
	s.mux.HandleFunc("GET /api/stream", s.stream)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// messages encodes structure and values once per change, shared by all clients.
func (s *Server) messages(st source.State) (structure, values []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.structMsg == nil || s.structRev != st.Rev {
		s.structRev, s.structMsg = st.Rev, encodeStructure(st)
	}
	if s.valuesMsg == nil || s.valuesSeq != st.Seq {
		s.valuesSeq, s.valuesMsg = st.Seq, encodeValues(st)
	}
	return s.structMsg, s.valuesMsg
}

// stream sends "status", "structure" and "values" events. The optional
// interval query parameter (ms) throttles values; 0 means every new sample.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	interval, _ := strconv.Atoi(r.URL.Query().Get("interval"))
	refresh := min(time.Duration(max(interval, 0))*time.Millisecond, maxRefresh)

	rc := http.NewResponseController(w)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")

	send := func(event string, data []byte) bool {
		rc.SetWriteDeadline(time.Now().Add(writeTimeout))
		if event != "" {
			w.Write([]byte("event: " + event + "\n"))
		}
		w.Write([]byte("data: "))
		w.Write(data)
		_, err := w.Write([]byte("\n\n"))
		return err == nil && rc.Flush() == nil
	}
	ping := func() bool {
		rc.SetWriteDeadline(time.Now().Add(writeTimeout))
		_, err := w.Write([]byte(": ping\n\n"))
		return err == nil && rc.Flush() == nil
	}

	var (
		sentStatus source.Status
		sentErr    string
		sentRev    uint64
		sentSeq    uint64
		sentNames  uint64
		sentPeriod time.Duration
		lastValues time.Time
		pending    <-chan time.Time
		alive      = time.NewTicker(keepAlive)
		first      = true
	)
	defer alive.Stop()
	w.Write([]byte("retry: 2000\n\n"))

	for {
		st, changed := s.src.Current()
		period := time.Duration(0)
		if st.Snapshot != nil {
			period = st.Snapshot.PollingPeriod
		}
		if first || st.Status != sentStatus || st.Err != sentErr || period != sentPeriod {
			if !send("status", encodeStatus(st)) {
				return
			}
			sentStatus, sentErr, sentPeriod, first = st.Status, st.Err, period, false
		}
		if st.Status == source.StatusOK {
			structure, values := s.messages(st)
			if st.Rev != sentRev {
				if !send("structure", structure) {
					return
				}
				sentRev, sentSeq, sentNames = st.Rev, 0, st.Names
			}
			if st.Names != sentNames {
				if !send("names", encodeNames(st)) {
					return
				}
				sentNames = st.Names
			}
			if st.Seq != sentSeq {
				if wait := refresh - time.Since(lastValues); wait > 0 {
					if pending == nil {
						pending = time.After(wait)
					}
				} else {
					if !send("values", values) {
						return
					}
					sentSeq, lastValues, pending = st.Seq, time.Now(), nil
				}
			}
		}

		select {
		case <-r.Context().Done():
			return
		case <-changed:
		case <-pending:
			pending = nil
		case <-alive.C:
			if !ping() {
				return
			}
		}
	}
}
