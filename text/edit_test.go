// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

import "testing"

func applyToString(e *EditSet, s string) string {
	result, err := ApplyToString(e, s)
	if err != nil {
		return "ERROR: " + err.Error()
	} else {
		return result
	}
}

func TestSizeChange(t *testing.T) {
	es := NewEditSet()
	es.Add(Extent{2, 5}, "x") // replace 5 bytes with 1 (-4)
	es.Add(Extent{7, 0}, "6") // add 1 byte
	if es.SizeChange() != -3 {
		t.Fatalf("SizeChange: expected -3, got %d", es.SizeChange())
	}

	es = NewEditSet()
	hello := "こんにちは"
	es.Add(Extent{6, 5}, hello)
	chg := len(hello) - 5
	if es.SizeChange() != int64(chg) {
		t.Fatalf("SizeChange: chged %d, got %d", chg, es.SizeChange())
	}
}

func TestEditString(t *testing.T) {
	es := NewEditSet()
	assertEquals("", es.String(), t)

	es.Add(Extent{5, 6}, "x")
	es.Add(Extent{1, 2}, "y")
	es.Add(Extent{3, 1}, "z")
	assertEquals(`Replace offset 1, length 2 with "y"
Replace offset 3, length 1 with "z"
Replace offset 5, length 6 with "x"
`, es.String(), t)
}

func TestOverlap(t *testing.T) {
	type test struct {
		offset, length  int
		overlapExpected bool // Does this overlap Extent{3,4}?
	}

	//                                                   123456789
	// Which intervals should overlap Extent{3,4}?   |--|
	tests := []test{
		test{2, 1, false}, // Regions starting to the left of offset 3
		test{2, 2, true},
		test{3, 0, false}, // Regions starting inside the interval
		test{3, 1, true},
		test{3, 4, true},
		test{3, 6, true},
		test{4, 1, true},
		test{4, 3, true},
		test{4, 9, true},
		test{6, 0, true},
		test{6, 1, true},
		test{6, 7, true},
		test{7, 0, false}, // Regions to the right of the interval
		test{7, 3, false},
	}

	for _, tst := range tests {
		es := NewEditSet()
		es.Add(Extent{3, 4}, "x")
		edit := Extent{tst.offset, tst.length}
		err := es.Add(edit, "z")
		if tst.overlapExpected != (err != nil) {
			t.Fatalf("Overlapping edit %s undetected", edit)
		}
	}
}

func TestEditApply(t *testing.T) {
	input := "0123456789"

	es := NewEditSet()
	assertEquals(input, applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{0, 0}, "AAA")
	assertEquals("AAA0123456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{0, 2}, "AAA")
	assertEquals("AAA23456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{3, 2}, "")
	assertEquals("01256789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{8, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	err := es.Add(Extent{-1, 3}, "")
	assertTrue(err != nil, t)
	//assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{12, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{2, 0}, "A")
	es.Add(Extent{8, 1}, "B")
	es.Add(Extent{4, 0}, "C")
	es.Add(Extent{6, 2}, "D")
	assertEquals("01A23C45DB9", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(Extent{0, 0}, "ABC")
	assertEquals("ABC", applyToString(es, ""), t)

	es = NewEditSet()
	es.Add(Extent{0, 3}, "")
	assertEquals("", applyToString(es, "ABC"), t)

	es = NewEditSet()
	es.Add(Extent{0, 0}, "")
	assertEquals("", applyToString(es, ""), t)
}
