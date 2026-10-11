package group

import (
	"fmt"
	"reflect"
	"testing"
)

func core(format string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(format, i)
	}
	return out
}

func coreThreads(format string, n int) []string {
	var out []string
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf(format, i, 0), fmt.Sprintf(format, i, 1))
	}
	return out
}

func cat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func same(n int, unit string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = unit
	}
	return out
}

func TestInferCPU(t *testing.T) {
	perf := []string{"1/1", "5/6", "4/5", "2/3"}
	clocks := make([]string, 4)
	for i := range clocks {
		clocks[i] = fmt.Sprintf("Core %d Clock (perf #%s)", i, perf[i])
	}
	labels := cat(core("Core %d VID", 4), clocks, []string{"Bus Clock"}, coreThreads("Core %d T%d Effective Clock", 4), []string{"Average Effective Clock"})
	got := Infer(labels, same(len(labels), "x"))
	want := []Group{
		{Name: "Core VID", Start: 0, Count: 4},
		{Name: "Core Clock", Start: 4, Count: 4},
		{Name: "Core Effective Clock", Start: 9, Count: 8},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestInferSplitsRestartedIndex(t *testing.T) {
	labels := cat([]string{"Package C6 Residency"}, core("Core %d C0 Residency", 3), core("Core %d C1 Residency", 3), core("Core %d C6 Residency", 3))
	got := Infer(labels, same(len(labels), "%"))
	want := []Group{
		{Name: "Core C0 Residency", Start: 1, Count: 3},
		{Name: "Core C1 Residency", Start: 4, Count: 3},
		{Name: "Core C6 Residency", Start: 7, Count: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestInferGluedIndexAndLanes(t *testing.T) {
	labels := []string{"CPU (Tctl/Tdie)", "Core0 (CCD1)", "Core1 (CCD1)", "Core2 (CCD2)", "Core3 (CCD2)", "L3 Cache (CCD1)", "L3 Cache (CCD2)", "PCIe Lane 0 Errors", "PCIe Lane 1 Errors", "PCIe Lane 2 Errors"}
	got := Infer(labels, same(len(labels), "°C"))
	want := []Group{
		{Name: "Core", Start: 1, Count: 4},
		{Name: "PCIe Lane Errors", Start: 7, Count: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestInferLeavesNonIndexRunsAlone(t *testing.T) {
	cases := map[string][]string{
		"unordered":   {"VIN5", "VIN6", "VIN2", "VIN3", "VIN7", "VIN9"},
		"pairs":       {"GPU Memory A0 Temperature", "GPU Memory A1 Temperature", "GPU Memory B0 Temperature", "GPU Memory B1 Temperature"},
		"too short":   {"GPU Fan1", "GPU Fan2"},
		"no number":   {"GPU Clock", "GPU Memory Clock"},
		"constant":    {"Temp3", "Temp3", "Temp3"},
		"starts high": {"Temp7", "Temp8", "Temp9"},
	}
	for name, labels := range cases {
		if got := Infer(labels, same(len(labels), "V")); len(got) != 0 {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

func TestInferRequiresSameUnit(t *testing.T) {
	labels := core("Core %d Power", 4)
	units := []string{"W", "W", "W", "%"}
	want := []Group{{Name: "Core Power", Start: 0, Count: 3}}
	if got := Infer(labels, units); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
}
