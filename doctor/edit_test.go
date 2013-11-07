package doctor

import (
	"testing"
)

const FILENAME = "-"

func applyToString(e EditSet, s string) string {
	result, err := e.ApplyToString(FILENAME, s)
	if err != nil {
		return "ERROR: " + err.Error()
	} else {
		return result
	}
}

func TestEditString(t *testing.T) {
	es := NewEditSet()
	assertEquals("", es.String(), t)

	es.Add(FILENAME, OffsetLength{5, 6}, "x")
	es.Add(FILENAME, OffsetLength{1, 2}, "y")
	es.Add(FILENAME, OffsetLength{3, 4}, "z")
	assertEquals(`Edits for -:
    Replace offset 1, length 2 with "y"
    Replace offset 3, length 4 with "z"
    Replace offset 5, length 6 with "x"
`, es.String(), t)
}

func TestEditApply(t *testing.T) {
	input := "0123456789"

	es := NewEditSet()
	assertEquals(input, applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 0}, "AAA")
	assertEquals("AAA0123456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 2}, "AAA")
	assertEquals("AAA23456789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{3, 2}, "")
	assertEquals("01256789", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{8, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{-1, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{12, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{2, 0}, "A")
	es.Add(FILENAME, OffsetLength{8, 1}, "B")
	es.Add(FILENAME, OffsetLength{4, 0}, "C")
	es.Add(FILENAME, OffsetLength{6, 2}, "D")
	assertEquals("01A23C45DB9", applyToString(es, input), t)
}
