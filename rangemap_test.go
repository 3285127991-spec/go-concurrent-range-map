package rangemap_test

import (
	"errors"
	"math"
	"sync"
	"testing"

	"rangemap"
)

func wantIntervals(t *testing.T, m *rangemap.RangeMap, want ...rangemap.Interval) {
	t.Helper()
	got := m.Snapshot()
	if len(got) != len(want) {
		t.Fatalf("Snapshot() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Snapshot() = %v, want %v", got, want)
		}
	}
	if m.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", m.Len(), len(want))
	}
}

func wantGet(t *testing.T, m *rangemap.RangeMap, point int64, value string, ok bool) {
	t.Helper()
	v, found := m.Get(point)
	if v != value || found != ok {
		t.Fatalf("Get(%d) = %q,%v, want %q,%v", point, v, found, value, ok)
	}
}

func iv(start, end int64, value string) rangemap.Interval {
	return rangemap.Interval{Start: start, End: end, Value: value}
}

func TestAssignAndGet(t *testing.T) {
	m := rangemap.New()
	if err := m.Assign(0, 10, "a"); err != nil {
		t.Fatal(err)
	}
	wantIntervals(t, m, iv(0, 10, "a"))
	wantGet(t, m, 0, "a", true)
	wantGet(t, m, 9, "a", true)
	wantGet(t, m, 10, "", false)
	wantGet(t, m, -1, "", false)

	for _, r := range [][2]int64{{5, 5}, {7, 3}} {
		if err := m.Assign(r[0], r[1], "x"); !errors.Is(err, rangemap.ErrInvalidRange) {
			t.Fatalf("Assign(%d,%d) err = %v, want ErrInvalidRange", r[0], r[1], err)
		}
		if err := m.Delete(r[0], r[1]); !errors.Is(err, rangemap.ErrInvalidRange) {
			t.Fatalf("Delete(%d,%d) err = %v, want ErrInvalidRange", r[0], r[1], err)
		}
	}
	if err := m.Assign(0, 1, ""); !errors.Is(err, rangemap.ErrEmptyValue) {
		t.Fatalf("Assign with empty value err = %v, want ErrEmptyValue", err)
	}
	wantIntervals(t, m, iv(0, 10, "a"))
}

func TestAssignOverwriteSplits(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 10, "a")
	mustAssign(t, m, 3, 7, "b")
	wantIntervals(t, m, iv(0, 3, "a"), iv(3, 7, "b"), iv(7, 10, "a"))
	wantGet(t, m, 2, "a", true)
	wantGet(t, m, 3, "b", true)
	wantGet(t, m, 6, "b", true)
	wantGet(t, m, 7, "a", true)
}

func TestAssignAcrossMultipleIntervals(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 2, "a")
	mustAssign(t, m, 2, 4, "b")
	mustAssign(t, m, 4, 6, "c")
	mustAssign(t, m, 6, 8, "d")
	wantIntervals(t, m, iv(0, 2, "a"), iv(2, 4, "b"), iv(4, 6, "c"), iv(6, 8, "d"))

	mustAssign(t, m, 1, 7, "x")
	wantIntervals(t, m, iv(0, 1, "a"), iv(1, 7, "x"), iv(7, 8, "d"))
	wantGet(t, m, 0, "a", true)
	wantGet(t, m, 1, "x", true)
	wantGet(t, m, 6, "x", true)
	wantGet(t, m, 7, "d", true)
}

func TestAdjacentSameValueMerge(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 5, "a")
	mustAssign(t, m, 5, 10, "a") // touching, same value: merge
	wantIntervals(t, m, iv(0, 10, "a"))

	mustAssign(t, m, 2, 8, "b")
	wantIntervals(t, m, iv(0, 2, "a"), iv(2, 8, "b"), iv(8, 10, "a"))

	mustAssign(t, m, 2, 8, "a") // bridges and merges both neighbours
	wantIntervals(t, m, iv(0, 10, "a"))

	mustAssign(t, m, 20, 25, "x")
	mustAssign(t, m, 25, 30, "y") // touching but different value: no merge
	wantIntervals(t, m, iv(0, 10, "a"), iv(20, 25, "x"), iv(25, 30, "y"))
}

func TestDeleteSplitAndAcross(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 10, "a")
	mustDelete(t, m, 3, 7) // split one interval into two
	wantIntervals(t, m, iv(0, 3, "a"), iv(7, 10, "a"))

	mustAssign(t, m, 3, 5, "b")
	mustAssign(t, m, 5, 7, "c")
	wantIntervals(t, m, iv(0, 3, "a"), iv(3, 5, "b"), iv(5, 7, "c"), iv(7, 10, "a"))

	mustDelete(t, m, 2, 8) // delete across several intervals
	wantIntervals(t, m, iv(0, 2, "a"), iv(8, 10, "a"))

	mustDelete(t, m, 100, 200) // no overlap: no-op
	mustDelete(t, m, 0, 10)    // delete everything
	wantIntervals(t, m)
	wantGet(t, m, 5, "", false)
}

func TestBoundaries(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 10, 20, "v")
	wantGet(t, m, 9, "", false)
	wantGet(t, m, 10, "v", true) // start inclusive
	wantGet(t, m, 19, "v", true)
	wantGet(t, m, 20, "", false) // end exclusive

	mustAssign(t, m, math.MinInt64, math.MinInt64+100, "lo")
	mustAssign(t, m, math.MinInt64+100, math.MinInt64+200, "lo") // merge at the edge
	wantGet(t, m, math.MinInt64, "lo", true)
	wantGet(t, m, math.MinInt64+199, "lo", true)
	wantGet(t, m, math.MinInt64+200, "", false)

	mustAssign(t, m, math.MaxInt64-100, math.MaxInt64, "hi")
	wantGet(t, m, math.MaxInt64-1, "hi", true)
	wantGet(t, m, math.MaxInt64, "", false)
	wantIntervals(t, m,
		iv(math.MinInt64, math.MinInt64+200, "lo"),
		iv(10, 20, "v"),
		iv(math.MaxInt64-100, math.MaxInt64, "hi"),
	)
}

func TestSnapshotIndependence(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 5, "a")
	mustAssign(t, m, 5, 10, "b")

	snap := m.Snapshot()
	want := []rangemap.Interval{iv(0, 5, "a"), iv(5, 10, "b")}
	if len(snap) != 2 || snap[0] != want[0] || snap[1] != want[1] {
		t.Fatalf("Snapshot() = %v, want %v", snap, want)
	}

	// Mutating the map must not change an earlier snapshot.
	mustAssign(t, m, 0, 10, "z")
	mustDelete(t, m, 3, 4)
	if snap[0] != want[0] || snap[1] != want[1] {
		t.Fatalf("snapshot changed after map mutation: %v", snap)
	}

	// Mutating a snapshot must not change the map.
	snap2 := m.Snapshot()
	snap2[0].Value = "corrupted"
	snap2[0].End = 999
	if got := m.Snapshot(); got[0].Value == "corrupted" || got[0].End == 999 {
		t.Fatalf("map changed after snapshot mutation: %v", got)
	}

	if got := rangemap.New().Snapshot(); len(got) != 0 {
		t.Fatalf("empty Snapshot() = %v, want empty", got)
	}
}

func TestClose(t *testing.T) {
	m := rangemap.New()
	mustAssign(t, m, 0, 10, "a")
	m.Close()
	m.Close() // idempotent, must not panic

	if err := m.Assign(1, 2, "b"); !errors.Is(err, rangemap.ErrClosed) {
		t.Fatalf("Assign after Close err = %v, want ErrClosed", err)
	}
	if err := m.Delete(1, 2); !errors.Is(err, rangemap.ErrClosed) {
		t.Fatalf("Delete after Close err = %v, want ErrClosed", err)
	}
	// Reads still work and see the pre-close state.
	wantGet(t, m, 5, "a", true)
	wantIntervals(t, m, iv(0, 10, "a"))
}

func TestConcurrent(t *testing.T) {
	m := rangemap.New()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				start := int64((g*7 + i) % 40)
				switch i % 4 {
				case 0:
					_ = m.Assign(start, start+10, "v")
				case 1:
					_ = m.Delete(start, start+5)
				case 2:
					_, _ = m.Get(start)
				case 3:
					_ = m.Snapshot()
					_ = m.Len()
				}
			}
		}(g)
	}
	wg.Wait()

	// The map must stay normalized: sorted, non-overlapping, merged.
	ivs := m.Snapshot()
	for i, cur := range ivs {
		if cur.Start >= cur.End {
			t.Fatalf("invalid interval %v", cur)
		}
		if cur.Value == "" {
			t.Fatalf("empty value in %v", cur)
		}
		if i == 0 {
			continue
		}
		prev := ivs[i-1]
		if prev.End > cur.Start {
			t.Fatalf("overlapping intervals %v and %v", prev, cur)
		}
		if prev.End == cur.Start && prev.Value == cur.Value {
			t.Fatalf("adjacent equal-valued intervals %v and %v", prev, cur)
		}
	}
	// Get must agree with the snapshot on every probed point.
	for p := int64(-1); p <= 50; p++ {
		wantV, wantOK := "", false
		for _, cur := range ivs {
			if cur.Start <= p && p < cur.End {
				wantV, wantOK = cur.Value, true
				break
			}
		}
		if v, ok := m.Get(p); v != wantV || ok != wantOK {
			t.Fatalf("Get(%d) = %q,%v, want %q,%v", p, v, ok, wantV, wantOK)
		}
	}
}

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
