// Package rangemap provides a concurrency-safe map from half-open
// int64 intervals [start, end) to non-empty string values.
//
// The stored intervals are always normalized: sorted by start,
// non-overlapping, and with no adjacent intervals sharing the same
// value. Assign and Delete keep this invariant by splitting existing
// intervals where needed and merging equal-valued neighbours.
package rangemap

import (
	"errors"
	"sort"
	"sync"
)

// Interval is a half-open range [Start, End) mapped to Value.
type Interval struct {
	Start int64
	End   int64
	Value string
}

var (
	// ErrInvalidRange is returned when start >= end.
	ErrInvalidRange = errors.New("rangemap: start must be less than end")
	// ErrEmptyValue is returned when value is empty.
	ErrEmptyValue = errors.New("rangemap: value must be non-empty")
	// ErrClosed is returned by Assign and Delete after Close.
	ErrClosed = errors.New("rangemap: closed")
)

// RangeMap maps non-overlapping intervals to values.
// The zero value is ready to use.
type RangeMap struct {
	mu     sync.RWMutex
	ivals  []Interval // sorted, non-overlapping, no equal-valued neighbours
	closed bool
}

// New returns an empty RangeMap.
func New() *RangeMap {
	return &RangeMap{}
}

// Assign maps [start, end) to value, overwriting whatever was there.
// Existing intervals are split at the range boundaries when needed and
// adjacent intervals with the same value are merged.
func (m *RangeMap) Assign(start, end int64, value string) error {
	if start >= end {
		return ErrInvalidRange
	}
	if value == "" {
		return ErrEmptyValue
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}

	lo, hi := m.overlap(start, end)

	// Segments replacing ivals[lo:hi]: the surviving left/right pieces
	// of the boundary intervals plus the new middle interval.
	segs := make([]Interval, 0, 3)
	if lo < hi {
		if first := m.ivals[lo]; first.Start < start {
			segs = append(segs, Interval{first.Start, start, first.Value})
		}
	}
	segs = append(segs, Interval{start, end, value})
	if lo < hi {
		if last := m.ivals[hi-1]; last.End > end {
			segs = append(segs, Interval{end, last.End, last.Value})
		}
	}
	segs = mergeAdjacent(segs)

	// Merge with the untouched neighbours of the replaced range.
	if lo > 0 {
		if prev := m.ivals[lo-1]; prev.End == segs[0].Start && prev.Value == segs[0].Value {
			segs[0].Start = prev.Start
			lo--
		}
	}
	if hi < len(m.ivals) {
		if next := m.ivals[hi]; next.Start == segs[len(segs)-1].End && next.Value == segs[len(segs)-1].Value {
			segs[len(segs)-1].End = next.End
			hi++
		}
	}

	m.replace(lo, hi, segs)
	return nil
}

// Delete removes any mapping overlapping [start, end). Intervals that
// extend past the range are split so their remaining parts survive.
func (m *RangeMap) Delete(start, end int64) error {
	if start >= end {
		return ErrInvalidRange
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}

	lo, hi := m.overlap(start, end)
	if lo == hi {
		return nil
	}
	segs := make([]Interval, 0, 2)
	if first := m.ivals[lo]; first.Start < start {
		segs = append(segs, Interval{first.Start, start, first.Value})
	}
	if last := m.ivals[hi-1]; last.End > end {
		segs = append(segs, Interval{end, last.End, last.Value})
	}
	m.replace(lo, hi, segs)
	return nil
}

// Get returns the value whose interval contains point.
func (m *RangeMap) Get(point int64) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i := sort.Search(len(m.ivals), func(i int) bool { return m.ivals[i].End > point })
	if i < len(m.ivals) && m.ivals[i].Start <= point {
		return m.ivals[i].Value, true
	}
	return "", false
}

// Snapshot returns an independent, ordered copy of all intervals.
func (m *RangeMap) Snapshot() []Interval {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Interval, len(m.ivals))
	copy(out, m.ivals)
	return out
}

// Len returns the number of normalized intervals.
func (m *RangeMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.ivals)
}

// Close makes the map read-only: Assign and Delete fail with ErrClosed
// while Get, Snapshot and Len keep working. It is safe to call twice.
func (m *RangeMap) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

// overlap returns the index range [lo, hi) of intervals intersecting
// [start, end). The caller must hold the lock.
func (m *RangeMap) overlap(start, end int64) (lo, hi int) {
	// Ends ascend with Starts, so both binary searches are valid.
	lo = sort.Search(len(m.ivals), func(i int) bool { return m.ivals[i].End > start })
	hi = sort.Search(len(m.ivals), func(i int) bool { return m.ivals[i].Start >= end })
	return lo, hi
}

// replace substitutes ivals[lo:hi] with segs, copying into a fresh
// backing array so snapshots handed out earlier stay untouched.
func (m *RangeMap) replace(lo, hi int, segs []Interval) {
	out := make([]Interval, 0, len(m.ivals)-(hi-lo)+len(segs))
	out = append(out, m.ivals[:lo]...)
	out = append(out, segs...)
	out = append(out, m.ivals[hi:]...)
	m.ivals = out
}

// mergeAdjacent merges runs of touching intervals that share a value.
// The input must be sorted and non-overlapping.
func mergeAdjacent(ivs []Interval) []Interval {
	out := ivs[:0]
	for _, iv := range ivs {
		if n := len(out); n > 0 && out[n-1].End == iv.Start && out[n-1].Value == iv.Value {
			out[n-1].End = iv.End
		} else {
			out = append(out, iv)
		}
	}
	return out
}
