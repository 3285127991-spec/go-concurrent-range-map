// Package rangemap provides a concurrency-safe map from half-open int64
// ranges [start, end) to non-empty string values.
//
// Intervals are always kept sorted by start and non-overlapping. Assign
// overwrites any existing coverage in its range, splitting intervals at
// the range boundaries when needed, and merges adjacent intervals that
// carry the same value. Delete removes coverage, likewise splitting
// intervals it partially overlaps.
package rangemap

import (
	"errors"
	"sort"
	"sync"
)

var (
	// ErrInvalidRange is returned when start >= end.
	ErrInvalidRange = errors.New("rangemap: start must be less than end")
	// ErrEmptyValue is returned when assigning an empty value.
	ErrEmptyValue = errors.New("rangemap: value must be non-empty")
	// ErrClosed is returned by Assign and Delete after Close.
	ErrClosed = errors.New("rangemap: map is closed")
)

// Interval is a half-open range [Start, End) mapped to Value.
type Interval struct {
	Start int64
	End   int64
	Value string
}

// RangeMap is a concurrency-safe range-to-value map.
// The zero value is ready to use.
type RangeMap struct {
	mu        sync.RWMutex
	intervals []Interval // sorted by Start, non-overlapping, no adjacent equal values
	closed    bool
}

// Assign maps [start, end) to value, overwriting any existing coverage
// in that range. Adjacent intervals with the same value are merged.
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
	m.intervals = m.cut(start, end)
	pos := sort.Search(len(m.intervals), func(i int) bool {
		return m.intervals[i].End > start
	})
	m.intervals = append(m.intervals, Interval{})
	copy(m.intervals[pos+1:], m.intervals[pos:])
	m.intervals[pos] = Interval{Start: start, End: end, Value: value}
	m.mergeAround(pos)
	return nil
}

// Delete removes any coverage in [start, end), splitting intervals it
// partially overlaps.
func (m *RangeMap) Delete(start, end int64) error {
	if start >= end {
		return ErrInvalidRange
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.intervals = m.cut(start, end)
	return nil
}

// Get returns the value whose interval contains point.
func (m *RangeMap) Get(point int64) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i := sort.Search(len(m.intervals), func(i int) bool {
		return m.intervals[i].End > point
	})
	if i < len(m.intervals) && m.intervals[i].Start <= point {
		return m.intervals[i].Value, true
	}
	return "", false
}

// Snapshot returns an independent ordered copy of all intervals.
func (m *RangeMap) Snapshot() []Interval {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Interval, len(m.intervals))
	copy(out, m.intervals)
	return out
}

// Len returns the number of normalized intervals.
func (m *RangeMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.intervals)
}

// Close rejects future Assign and Delete calls. Get, Snapshot and Len
// remain usable. Close is idempotent.
func (m *RangeMap) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
}

// cut returns a copy of m.intervals with [start, end) removed, splitting
// partially overlapping intervals. The caller must hold the write lock.
func (m *RangeMap) cut(start, end int64) []Interval {
	kept := make([]Interval, 0, len(m.intervals)+1)
	for _, iv := range m.intervals {
		if iv.End <= start || iv.Start >= end {
			kept = append(kept, iv)
			continue
		}
		if iv.Start < start {
			kept = append(kept, Interval{Start: iv.Start, End: start, Value: iv.Value})
		}
		if iv.End > end {
			kept = append(kept, Interval{Start: end, End: iv.End, Value: iv.Value})
		}
	}
	return kept
}

// mergeAround merges the interval at pos with adjacent intervals that
// touch it and carry the same value. The caller must hold the write lock.
func (m *RangeMap) mergeAround(pos int) {
	for pos+1 < len(m.intervals) &&
		m.intervals[pos].End == m.intervals[pos+1].Start &&
		m.intervals[pos].Value == m.intervals[pos+1].Value {
		m.intervals[pos].End = m.intervals[pos+1].End
		m.intervals = append(m.intervals[:pos+1], m.intervals[pos+2:]...)
	}
	if pos > 0 &&
		m.intervals[pos-1].End == m.intervals[pos].Start &&
		m.intervals[pos-1].Value == m.intervals[pos].Value {
		m.intervals[pos-1].End = m.intervals[pos].End
		m.intervals = append(m.intervals[:pos], m.intervals[pos+1:]...)
	}
}
