// Command rangemap-demo demonstrates the rangemap library: overwrite,
// split, merge of adjacent equal values, delete, and reads after Close.
package main

import (
	"fmt"

	"rangemap"
)

func show(label string, m *rangemap.RangeMap) {
	fmt.Printf("%-26s", label)
	for _, iv := range m.Snapshot() {
		fmt.Printf("[%d,%d)=%q ", iv.Start, iv.End, iv.Value)
	}
	fmt.Printf(" Len=%d\n", m.Len())
}

func main() {
	m := new(rangemap.RangeMap)

	must(m.Assign(0, 10, "a"))
	show("Assign [0,10)=\"a\"", m)

	must(m.Assign(3, 7, "b"))
	show("Assign [3,7)=\"b\" split", m)

	must(m.Assign(7, 10, "b"))
	show("Assign [7,10)=\"b\" merge", m)

	must(m.Delete(4, 6))
	show("Delete [4,6)", m)

	if v, ok := m.Get(5); ok {
		fmt.Printf("Get(5) = %q, %v\n", v, ok)
	} else {
		fmt.Println("Get(5) = not found (deleted)")
	}
	if v, ok := m.Get(8); ok {
		fmt.Printf("Get(8) = %q, %v\n", v, ok)
	}

	m.Close()
	m.Close() // idempotent
	fmt.Println("Close() called twice")

	fmt.Println("Assign after Close:", m.Assign(0, 1, "x"))
	fmt.Println("Delete after Close:", m.Delete(0, 1))

	if v, ok := m.Get(8); ok {
		fmt.Printf("Get(8) after Close = %q, %v\n", v, ok)
	}
	show("Snapshot after Close", m)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
