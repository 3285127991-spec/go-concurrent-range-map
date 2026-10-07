package rangemap_test

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"rangemap"
)

func mustAssign(t *testing.T, m *rangemap.RangeMap, start, end int64, value string) {
	t.Helper()
	if err := m.Assign(start, end, value); err != nil {
		t.Fatalf("Assign(%d,%d,%q): %v", start, end, value, err)
	}
}

func mustDelete(t *testing.T, m *rangemap.RangeMap, start, end int64) {
	t.Helper()
	if err := m.Delete(start, end); err != nil {
		t.Fatalf("Delete(%d,%d): %v", start, end, err)
	}
}

func checkSnapshot(t *testing.T, m *rangemap.RangeMap, want []rangemap.Interval) {
	t.Helper()
	got := m.Snapshot()
	if !slices.Equal(got, want) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, want)
	}
	if m.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", m.Len(), len(want))
	}
}

func checkGet(t *testing.T, m *rangemap.RangeMap, point int64, wantValue string, wantOK bool) {
	t.Helper()
	v, ok := m.Get(point)
	if ok != wantOK || (ok && v != wantValue) {
		t.Fatalf("Get(%d) = %q,%v, want %q,%v", point, v, ok, wantValue, wantOK)
	}
}

func TestAssignAndGet(t *testing.T) {
	m := new(rangemap.RangeMap)
	checkGet(t, m, 5, "", false)

	mustAssign(t, m, 0, 10, "a")
	checkGet(t, m, 0, "a", true)
	checkGet(t, m, 5, "a", true)
	checkGet(t, m, 9, "a", true)
	checkGet(t, m, 10, "", false)
	checkGet(t, m, -1, "", false)
	checkSnapshot(t, m, []rangemap.Interval{{Start: 0, End: 10, Value: "a"}})
}

func TestAssignOverwriteSplits(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 10, "a")
	mustAssign(t, m, 3, 7, "b")
	checkSnapshot(t, m, []rangemap.Interval{
		{Start: 0, End: 3, Value: "a"},
		{Start: 3, End: 7, Value: "b"},
		{Start: 7, End: 10, Value: "a"},
	})
	checkGet(t, m, 2, "a", true)
	checkGet(t, m, 3, "b", true)
	checkGet(t, m, 6, "b", true)
	checkGet(t, m, 7, "a", true)
}

func TestAssignAcrossMultipleIntervals(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 5, "a")
	mustAssign(t, m, 5, 10, "b")
	mustAssign(t, m, 10, 15, "c")
	mustAssign(t, m, 3, 12, "x")
	checkSnapshot(t, m, []rangemap.Interval{
		{Start: 0, End: 3, Value: "a"},
		{Start: 3, End: 12, Value: "x"},
		{Start: 12, End: 15, Value: "c"},
	})
}

func TestMergeAdjacentSameValue(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 5, "a")
	mustAssign(t, m, 10, 15, "a")
	mustAssign(t, m, 5, 10, "a") // bridges both sides
	checkSnapshot(t, m, []rangemap.Interval{{Start: 0, End: 15, Value: "a"}})

	// Different values must stay separate even when touching.
	mustAssign(t, m, 15, 20, "b")
	checkSnapshot(t, m, []rangemap.Interval{
		{Start: 0, End: 15, Value: "a"},
		{Start: 15, End: 20, Value: "b"},
	})

	// Overwriting with the same value merges the whole range back.
	mustAssign(t, m, 10, 20, "a")
	checkSnapshot(t, m, []rangemap.Interval{{Start: 0, End: 20, Value: "a"}})
}

func TestDeleteSplitsAndSpans(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 10, "a")
	mustDelete(t, m, 3, 7) // split into left and right parts
	checkSnapshot(t, m, []rangemap.Interval{
		{Start: 0, End: 3, Value: "a"},
		{Start: 7, End: 10, Value: "a"},
	})
	checkGet(t, m, 5, "", false)

	m2 := new(rangemap.RangeMap)
	mustAssign(t, m2, 0, 5, "a")
	mustAssign(t, m2, 5, 10, "b")
	mustAssign(t, m2, 10, 15, "c")
	mustDelete(t, m2, 3, 12) // spans several intervals
	checkSnapshot(t, m2, []rangemap.Interval{
		{Start: 0, End: 3, Value: "a"},
		{Start: 12, End: 15, Value: "c"},
	})

	mustDelete(t, m2, 100, 200) // no-op outside coverage
	mustDelete(t, m2, 0, 15)    // delete everything
	checkSnapshot(t, m2, nil)
}

func TestBoundariesAndValidation(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 2, 4, "x")
	mustAssign(t, m, 4, 6, "y")

	// Half-open semantics: start inclusive, end exclusive.
	checkGet(t, m, 1, "", false)
	checkGet(t, m, 2, "x", true)
	checkGet(t, m, 3, "x", true)
	checkGet(t, m, 4, "y", true)
	checkGet(t, m, 6, "", false)

	for _, r := range [][2]int64{{5, 5}, {6, 5}} {
		if err := m.Assign(r[0], r[1], "z"); !errors.Is(err, rangemap.ErrInvalidRange) {
			t.Fatalf("Assign(%d,%d) err = %v, want ErrInvalidRange", r[0], r[1], err)
		}
		if err := m.Delete(r[0], r[1]); !errors.Is(err, rangemap.ErrInvalidRange) {
			t.Fatalf("Delete(%d,%d) err = %v, want ErrInvalidRange", r[0], r[1], err)
		}
	}
	if err := m.Assign(0, 1, ""); !errors.Is(err, rangemap.ErrEmptyValue) {
		t.Fatalf("Assign empty value err = %v, want ErrEmptyValue", err)
	}
	// Failed calls must not mutate the map.
	checkSnapshot(t, m, []rangemap.Interval{
		{Start: 2, End: 4, Value: "x"},
		{Start: 4, End: 6, Value: "y"},
	})
}

func TestSnapshotIndependence(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 10, "a")

	snap := m.Snapshot()
	mustAssign(t, m, 0, 10, "b")
	mustDelete(t, m, 2, 4)

	// The snapshot taken before the mutations is unchanged.
	want := []rangemap.Interval{{Start: 0, End: 10, Value: "a"}}
	if !slices.Equal(snap, want) {
		t.Fatalf("stale snapshot = %+v, want %+v", snap, want)
	}

	// Mutating the returned slice must not affect the map.
	snap[0].Value = "corrupted"
	checkGet(t, m, 5, "b", true)
}

func TestClose(t *testing.T) {
	m := new(rangemap.RangeMap)
	mustAssign(t, m, 0, 10, "a")

	m.Close()
	m.Close() // idempotent, must not panic

	if err := m.Assign(1, 2, "b"); !errors.Is(err, rangemap.ErrClosed) {
		t.Fatalf("Assign after Close err = %v, want ErrClosed", err)
	}
	if err := m.Delete(1, 2); !errors.Is(err, rangemap.ErrClosed) {
		t.Fatalf("Delete after Close err = %v, want ErrClosed", err)
	}

	// Reads still work after Close.
	checkGet(t, m, 5, "a", true)
	checkSnapshot(t, m, []rangemap.Interval{{Start: 0, End: 10, Value: "a"}})
}

func TestConcurrent(t *testing.T) {
	m := new(rangemap.RangeMap)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			base := id * 1000
			for i := int64(0); i < 20; i++ {
				s := base + i
				if err := m.Assign(s, s+10, "v"); err != nil {
					t.Errorf("Assign: %v", err)
				}
				m.Get(s)
				m.Snapshot()
				m.Len()
				if err := m.Delete(s, s+5); err != nil {
					t.Errorf("Delete: %v", err)
				}
			}
		}(int64(w))
	}
	wg.Wait()

	// Invariants: sorted, non-overlapping, normalized, valid.
	snap := m.Snapshot()
	for i, iv := range snap {
		if iv.Start >= iv.End || iv.Value == "" {
			t.Fatalf("invalid interval %+v", iv)
		}
		if i == 0 {
			continue
		}
		prev := snap[i-1]
		if prev.End > iv.Start {
			t.Fatalf("overlapping intervals %+v and %+v", prev, iv)
		}
		if prev.End == iv.Start && prev.Value == iv.Value {
			t.Fatalf("adjacent equal-value intervals not merged: %+v and %+v", prev, iv)
		}
	}
}
