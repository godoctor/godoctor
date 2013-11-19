// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

package doctor

import (
	"fmt"
	"strings"
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
	err := es.Add(FILENAME, OffsetLength{-1, 3}, "")
	assertTrue(err != nil, t)
	//assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{12, 3}, "")
	assertError(applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{2, 0}, "A")
	es.Add(FILENAME, OffsetLength{8, 1}, "B")
	es.Add(FILENAME, OffsetLength{4, 0}, "C")
	es.Add(FILENAME, OffsetLength{6, 2}, "D")
	assertEquals("01A23C45DB9", applyToString(es, input), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 0}, "ABC")
	assertEquals("ABC", applyToString(es, ""), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 3}, "")
	assertEquals("", applyToString(es, "ABC"), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 0}, "")
	assertEquals("", applyToString(es, ""), t)
}

func TestLineRdr(t *testing.T) {
	// Line2 starts at offset 10, Line3 at 20, etc.
	s := "Line1....\nLine2....\nLine3....\nLine4....\nLine5"
	r := newLineRdr(strings.NewReader(s))

	r.readLine()
	assertEquals("Line1....\n", r.line, t)
	assertTrue(r.lineOffset == 0, t)
	assertTrue(r.lineNum == 1, t)
	assertTrue(len(r.leadingCtxLines) == 0, t)

	r.readLine()
	assertEquals("Line2....\n", r.line, t)
	assertTrue(r.lineOffset == 10, t)
	assertTrue(r.lineNum == 2, t)
	assertTrue(len(r.leadingCtxLines) == 1, t)

	r.readLine()
	r.readLine()
	r.readLine()
	assertEquals("Line5", r.line, t)
	assertTrue(r.lineOffset == 40, t)
	assertTrue(r.lineNum == 5, t)
	assertTrue(len(r.leadingCtxLines) == num_ctx_lines, t)
	assertEquals("Line2....\n", r.leadingCtxLines[0], t)
	assertEquals("Line3....\n", r.leadingCtxLines[1], t)
	assertEquals("Line4....\n", r.leadingCtxLines[2], t)
}

func TestCreatePatch(t *testing.T) {
	// Line2 starts at offset 10, Line3 at 20, etc.
	s := "Line1....\nLine2....\nLine3....\nLine4"

	es := NewEditSet()
	p, err := es.CreatePatch(FILENAME, strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 0, t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 0}, "AAA")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 2}, "AAA")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{2, 5}, "AAA")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{2, 15}, "AAA")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	// Line n starts at offset (n-1)*5
	s2 := "1...\n2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n"
	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{20, 2}, "5555\n5!")
	es.Add(FILENAME, OffsetLength{40, 0}, "CCC")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s2))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 5, t)
	assertTrue(p.hunks[0].startLine == 2, t)
	assertTrue(p.hunks[0].numLines == 9, t)
	assertEquals("2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n", p.hunks[0].hunk.String(), t)
	assertTrue(len(p.hunks[0].edits) == 2, t)

	es = NewEditSet()
	es.Add(FILENAME, OffsetLength{0, 0}, "A")
	es.Add(FILENAME, OffsetLength{36, 0}, "B")
	p, err = es.CreatePatch(FILENAME, strings.NewReader(s2))
	fmt.Println(p)
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 2, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertTrue(p.hunks[0].numLines >= 4, t) // Actually 7
	assertTrue(len(p.hunks[0].edits) == 1, t)
	assertTrue(p.hunks[1].startOffset == 20, t)
	assertTrue(p.hunks[1].startLine == 5, t)
	assertTrue(p.hunks[1].numLines == 6, t)
	assertTrue(len(p.hunks[1].edits) == 1, t)
}

func TestUnifiedDiff(t *testing.T) {
	a := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7
Line 8
Line 9
Line 10`
	b := `Line 2
Line 3
Line 4
Line 5
Line 6 has changed
Line 7 has also changed
This is line 7.5
Line 8
Line 9
Line 10`
	edits := Diff("test.txt", a, b)
	t.Error(edits)
	s, _ := edits.ApplyToString("test.txt", a)
	assertEquals(b, s, t)
	t.Error(edits.CreatePatch("test.txt", strings.NewReader(a)))
}
