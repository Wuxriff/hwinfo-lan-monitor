// Command shmdump writes the current HWiNFO shared memory as JSON and, optionally, the raw bytes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
)

func main() {
	out := flag.String("o", "", "JSON output file (default stdout)")
	raw := flag.String("raw", "", "also write the raw view bytes to this file")
	flag.Parse()

	b, err := shm.ReadRaw()
	if err != nil {
		fail(err)
	}
	if *raw != "" {
		if err := os.WriteFile(*raw, b, 0o644); err != nil {
			fail(err)
		}
	}
	s, err := shm.Parse(b)
	if err != nil {
		fail(err)
	}
	j, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fail(err)
	}
	if *out == "" {
		os.Stdout.Write(append(j, '\n'))
		return
	}
	if err := os.WriteFile(*out, j, 0o644); err != nil {
		fail(err)
	}
	readings := 0
	for _, se := range s.Sensors {
		readings += len(se.Readings)
	}
	fmt.Fprintf(os.Stderr, "v%d rev%d period=%s sensors=%d readings=%d bytes=%d\n",
		s.Version, s.Revision, s.PollingPeriod, len(s.Sensors), readings, len(b))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "shmdump:", err)
	os.Exit(1)
}
