// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"testing"
)

func applyToString(e EditSet, s string) string {
	result, err := ApplyToString(e, s)
	if err != nil {
		return "ERROR: " + err.Error()
	} else {
		return result
	}
}

func TestEditString(t *testing.T) {
	es := NewEditSet()
	assertEquals("", es.String(), t)

	es.Add(OffsetLength{5, 6}, "x")
	es.Add(OffsetLength{1, 2}, "y")
	es.Add(OffsetLength{3, 4}, "z")
	assertEquals(`Replace offset 1, length 2 with "y"
Replace offset 3, length 4 with "z"
Replace offset 5, length 6 with "x"
`, es.String(), t)
}

func TestEditApply(t *testing.T) {
	input := "0123456789"

	es := NewEditSet()
	assertEquals(input, applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 0}, "AAA")
	assertEquals("AAA0123456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 2}, "AAA")
	assertEquals("AAA23456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{3, 2}, "")
	assertEquals("01256789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{8, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	err := es.Add(OffsetLength{-1, 3}, "")
	assertTrue(err != nil, t)
	//assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{12, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{2, 0}, "A")
	es.Add(OffsetLength{8, 1}, "B")
	es.Add(OffsetLength{4, 0}, "C")
	es.Add(OffsetLength{6, 2}, "D")
	assertEquals("01A23C45DB9", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 0}, "ABC")
	assertEquals("ABC", applyToString(es, ""), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 3}, "")
	assertEquals("", applyToString(es, "ABC"), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 0}, "")
	assertEquals("", applyToString(es, ""), t)
}
