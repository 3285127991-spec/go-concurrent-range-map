// Command rangemap-demo walks through the RangeMap features:
// overwrite, splitting, adjacent merge, delete, and reads after Close.
package main

import (
	"fmt"

	"rangemap"
)

func main() {
	m := rangemap.New()

	step("Assign [0,10) = \"a\"", m)
	must(m.Assign(0, 10, "a"))
	show(m)

	step("Assign [3,7) = \"b\"  -- overwrites the middle, splits \"a\"", m)
	must(m.Assign(3, 7, "b"))
	show(m)

	step("Assign [7,12) = \"b\"  -- adjacent equal values merge", m)
	must(m.Assign(7, 12, "b"))
	show(m)

	step("Assign [1,11) = \"c\"  -- overwrites across intervals", m)
	must(m.Assign(1, 11, "c"))
	show(m)

	step("Delete [2,9)  -- splits \"c\" into two parts", m)
	must(m.Delete(2, 9))
	show(m)

	v1, ok1 := m.Get(1)
	v5, ok5 := m.Get(5)
	fmt.Printf("Get(1) = %q,%v  Get(5) = %q,%v\n", v1, ok1, v5, ok5)

	fmt.Println("\nClose (twice, idempotent); reads keep working:")
	m.Close()
	m.Close()
	v9, ok9 := m.Get(9)
	fmt.Printf("Get(9) = %q,%v  Len = %d\n", v9, ok9, m.Len())
	show(m)
	if err := m.Assign(0, 1, "x"); err != nil {
		fmt.Printf("Assign after Close rejected: %v\n", err)
	}
	if err := m.Delete(0, 1); err != nil {
		fmt.Printf("Delete after Close rejected: %v\n", err)
	}
}

func step(msg string, m *rangemap.RangeMap) {
	fmt.Printf("\n> %s\n", msg)
}

func show(m *rangemap.RangeMap) {
	fmt.Printf("  Len=%d  intervals:", m.Len())
	for _, iv := range m.Snapshot() {
		fmt.Printf("  [%d,%d)=%q", iv.Start, iv.End, iv.Value)
	}
	fmt.Println()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
