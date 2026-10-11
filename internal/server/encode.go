package server

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/group"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/source"
)

type wireReading struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Unit  string `json:"unit"`
	Type  string `json:"type"`
}

type wireSensor struct {
	Key      string        `json:"key"`
	Name     string        `json:"name"`
	Readings []wireReading `json:"readings"`
	Groups   []group.Group `json:"groups"`
}

type wireStructure struct {
	Rev      uint64       `json:"rev"`
	PeriodMs int64        `json:"periodMs"`
	Sensors  []wireSensor `json:"sensors"`
}

type wireStatus struct {
	Status   source.Status `json:"status"`
	Error    string        `json:"error,omitempty"`
	PeriodMs int64         `json:"periodMs,omitempty"`
}

func encodeStructure(st source.State) []byte {
	s := st.Snapshot
	w := wireStructure{Rev: st.Rev, PeriodMs: s.PollingPeriod.Milliseconds(), Sensors: make([]wireSensor, len(s.Sensors))}
	for i, se := range s.Sensors {
		key := fmt.Sprintf("%x.%d", se.ID, se.Instance)
		ws := wireSensor{Key: key, Name: se.Name, Readings: make([]wireReading, len(se.Readings))}
		labels, units := make([]string, len(se.Readings)), make([]string, len(se.Readings))
		for j, r := range se.Readings {
			ws.Readings[j] = wireReading{Key: fmt.Sprintf("%s.%d", key, r.ID), Label: r.Label, Unit: r.Unit, Type: r.Type.String()}
			labels[j], units[j] = r.Label, r.Unit
		}
		ws.Groups = group.Infer(labels, units)
		if ws.Groups == nil {
			ws.Groups = []group.Group{}
		}
		w.Sensors[i] = ws
	}
	b, _ := json.Marshal(w)
	return b
}

// encodeValues writes Current/Min/Max/Avg of every Reading, in structure order, as one flat array.
func encodeValues(st source.State) []byte {
	b := make([]byte, 0, 64*1024)
	b = append(b, `{"seq":`...)
	b = strconv.AppendUint(b, st.Seq, 10)
	b = append(b, `,"t":`...)
	b = strconv.AppendInt(b, st.Snapshot.PollTime.Unix(), 10)
	b = append(b, `,"v":[`...)
	first := true
	for _, se := range st.Snapshot.Sensors {
		for _, r := range se.Readings {
			for _, v := range [4]float64{r.Value, r.Min, r.Max, r.Avg} {
				if !first {
					b = append(b, ',')
				}
				first = false
				b = appendNumber(b, v)
			}
		}
	}
	return append(b, "]}"...)
}

func appendNumber(b []byte, v float64) []byte {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return append(b, "null"...)
	}
	return strconv.AppendFloat(b, math.Round(v*1e4)/1e4, 'f', -1, 64)
}

func encodeStatus(st source.State) []byte {
	w := wireStatus{Status: st.Status, Error: st.Err}
	if st.Snapshot != nil {
		w.PeriodMs = st.Snapshot.PollingPeriod.Milliseconds()
	}
	b, _ := json.Marshal(w)
	return b
}

// encodeNames lists Sensor names in structure order, for sensors HWiNFO renames at runtime.
func encodeNames(st source.State) []byte {
	names := make([]string, len(st.Snapshot.Sensors))
	for i, se := range st.Snapshot.Sensors {
		names[i] = se.Name
	}
	b, _ := json.Marshal(names)
	return b
}
