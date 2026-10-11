// Package group infers Reading groups from the labels of one Sensor, because
// HWiNFO shared memory does not publish the groups its sensors window shows.
package group

import (
	"strconv"
	"strings"
)

const minMembers = 3

// Group is a run of consecutive Readings [Start, Start+Count) sharing one Group row.
type Group struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	Count int    `json:"count"`
}

type run struct{ start, end, num int }

type parsed struct {
	template string
	nums     []run
}

func parse(label string) parsed {
	var tpl strings.Builder
	var nums []run
	for i := 0; i < len(label); {
		if !isDigit(label[i]) {
			tpl.WriteByte(label[i])
			i++
			continue
		}
		j := i
		for j < len(label) && isDigit(label[j]) {
			j++
		}
		n, _ := strconv.Atoi(label[i:j])
		nums = append(nums, run{i, j, n})
		tpl.WriteByte('#')
		i = j
	}
	return parsed{tpl.String(), nums}
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// Infer finds runs of at least three consecutive Readings with the same unit whose
// labels differ only by numbers, where the first number is an index that starts
// at 0 or 1 and never steps by more than one (per core, per thread, per lane).
func Infer(labels, units []string) []Group {
	p := make([]parsed, len(labels))
	for i, l := range labels {
		p[i] = parse(l)
	}
	var out []Group
	for i := 0; i < len(labels); {
		if len(p[i].nums) == 0 || p[i].nums[0].num > 1 {
			i++
			continue
		}
		j, distinct := i+1, false
		for ; j < len(labels); j++ {
			if p[j].template != p[i].template || units[j] != units[i] {
				break
			}
			prev, cur := p[j-1].nums[0].num, p[j].nums[0].num
			if cur != prev && cur != prev+1 {
				break
			}
			distinct = distinct || cur != prev
		}
		if j-i >= minMembers && distinct {
			out = append(out, Group{Name: name(labels[i:j], p[i:j]), Start: i, Count: j - i})
			i = j
			continue
		}
		i++
	}
	return out
}

// name derives the Group row label from the first member by removing the parts
// that vary between members: whole parentheticals, short tokens such as "T0",
// and bare digits glued to a word such as "Core0".
func name(labels []string, p []parsed) string {
	first := labels[0]
	varying := make([]bool, len(first))
	for k, r := range p[0].nums {
		for _, q := range p[1:] {
			if q.nums[k].num != r.num {
				for b := r.start; b < r.end; b++ {
					varying[b] = true
				}
				break
			}
		}
	}

	deleted := make([]bool, len(first))
	for open := strings.IndexAny(first, "(["); open >= 0; {
		closeRel := strings.IndexAny(first[open:], ")]")
		if closeRel < 0 {
			break
		}
		end := open + closeRel + 1
		for b := open; b < end; b++ {
			if varying[b] {
				for d := open; d < end; d++ {
					deleted[d] = true
				}
				break
			}
		}
		next := strings.IndexAny(first[end:], "([")
		if next < 0 {
			break
		}
		open = end + next
	}

	var words []string
	for start := 0; start < len(first); {
		end := strings.IndexByte(first[start:], ' ')
		if end < 0 {
			end = len(first)
		} else {
			end += start
		}
		var kept strings.Builder
		hasVarying := false
		for b := start; b < end; b++ {
			switch {
			case deleted[b]:
			case varying[b]:
				hasVarying = true
			default:
				kept.WriteByte(first[b])
			}
		}
		if w := kept.String(); w != "" && !(hasVarying && len(w) <= 2) {
			words = append(words, w)
		}
		start = end + 1
	}
	return strings.Join(words, " ")
}
