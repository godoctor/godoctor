// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

package doctor

import (
	"math/rand"
	"strings"
	"testing"
)

var rng *rand.Rand = rand.New(rand.NewSource(99))

func testDiff(a, b string, t *testing.T) {
	diff := Diff(strings.Split(a, ""), strings.Split(b, ""))
	actual, err := ApplyToString(diff, a)
	failIfError(err, t)
	expected := b
	assertEquals(expected, actual, t)
}

func TestMyersPaperExample(t *testing.T) {
	testDiff("ABCABBA", "CBABAC", t)
}

func TestEmptyCases(t *testing.T) {
	testDiff("", "", t)
	testDiff("", "abcdefg", t)
	testDiff("abcdefg", "", t)
}

func TestNoChange(t *testing.T) {
	testDiff("a", "a", t)
	testDiff("a\nbcd", "a\nbcd", t)
}

func TestAdditionalDiffs(t *testing.T) {
	testDiff("abcdefg", "abcfg", t)
	testDiff("abcdefg", "defg", t)
	testDiff("abcdefg", "abcd", t)
	testDiff("abcdefg", "ag", t)
	testDiff("abcdefg", "bcd", t)
	testDiff("abcdefg", "bdf", t)
	testDiff("abc", "defg", t)
	testDiff("abcdefg", "xyz", t)
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
	p, err := es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 0, t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 0}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 2}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(OffsetLength{2, 5}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	es = NewEditSet()
	es.Add(OffsetLength{2, 15}, "AAA")
	p, err = es.CreatePatch(strings.NewReader(s))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertEquals("Line1....\nLine2....\nLine3....\nLine4",
		p.hunks[0].hunk.String(), t)

	// Line n starts at offset (n-1)*5
	s2 := "1...\n2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n"
	es = NewEditSet()
	es.Add(OffsetLength{20, 2}, "5555\n5!")
	es.Add(OffsetLength{40, 0}, "CCC")
	p, err = es.CreatePatch(strings.NewReader(s2))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 1, t)
	assertTrue(p.hunks[0].startOffset == 5, t)
	assertTrue(p.hunks[0].startLine == 2, t)
	assertTrue(p.hunks[0].numLines == 10, t)
	assertEquals("2...\n3...\n4...\n5...\n6...\n7...\n8...\n9...\n0...\n", p.hunks[0].hunk.String(), t)
	assertTrue(len(p.hunks[0].edits) == 2, t)

	es = NewEditSet()
	es.Add(OffsetLength{0, 0}, "A")
	es.Add(OffsetLength{36, 0}, "B")
	p, err = es.CreatePatch(strings.NewReader(s2))
	assertTrue(err == nil, t)
	assertTrue(len(p.hunks) == 2, t)
	assertTrue(p.hunks[0].startOffset == 0, t)
	assertTrue(p.hunks[0].startLine == 1, t)
	assertTrue(p.hunks[0].numLines >= 4, t) // Actually 7
	assertTrue(len(p.hunks[0].edits) == 1, t)
	assertTrue(p.hunks[1].startOffset == 20, t)
	assertTrue(p.hunks[1].startLine == 5, t)
	assertTrue(p.hunks[1].numLines == 7, t)
	assertTrue(len(p.hunks[1].edits) == 1, t)
}

func testUnifiedDiff(a string, b string, expected string, t *testing.T) {
	edits := Diff(strings.SplitAfter(a, "\n"), strings.SplitAfter(b, "\n"))
	s, _ := ApplyToString(edits, a)
	assertEquals(b, s, t)
	patch, _ := edits.CreatePatch(strings.NewReader(a))
	assertEquals(expected, patch.String(), t)
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
	expected := `--- filename
+++ filename
@@ -1,10 +1,10 @@
-Line 1
 Line 2
 Line 3
 Line 4
 Line 5
-Line 6
-Line 7
+Line 6 has changed
+Line 7 has also changed
+This is line 7.5
 Line 8
 Line 9
 Line 10`
	testUnifiedDiff(a, b, expected, t)
}

func TestUnifiedDiffLastLine(t *testing.T) {
	a := `Line 1
Line 2
Line 3
`
	b := `Line 1
Line 2
Line 33`
	expected := `--- filename
+++ filename
@@ -1,3 +1,3 @@
 Line 1
 Line 2
-Line 3
+Line 33
\ No newline at end of file
`
	testUnifiedDiff(a, b, expected, t)
}

func TestUnifiedDiffNoLFAtEnd(t *testing.T) {
	a := `Line 1
Line 2
Line 3`
	b := `Line 1
Line 2
Line 33`
	expected := `--- filename
+++ filename
@@ -1,3 +1,3 @@
 Line 1
 Line 2
-Line 3
\ No newline at end of file
+Line 33
\ No newline at end of file
`
	testUnifiedDiff(a, b, expected, t)
}

func TestUnifiedDiffNoChange(t *testing.T) {
	a := `Line 1
Line 2
Line 3
`
	expected := ""
	testUnifiedDiff(a, a, expected, t)
}

func TestUnifiedDiffMultipleHunks(t *testing.T) {
	a := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7
Line 8
Line 9
Line 10
`
	b := `Line 1
Line 3
Line 4
Line 5
Line 6
Line 7
Line 8
Line 9
Line 9.5
Line 9.75
Line 10`
	expected := `--- filename
+++ filename
@@ -1,5 +1,4 @@
 Line 1
-Line 2
 Line 3
 Line 4
 Line 5
@@ -7,4 +6,6 @@
 Line 7
 Line 8
 Line 9
-Line 10
+Line 9.5
+Line 9.75
+Line 10
\ No newline at end of file
`
	testUnifiedDiff(a, b, expected, t)
}

func TestUnifiedDiffMaxIntraHunkCtx(t *testing.T) {
	a := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7
Line 8
Line 9
Line 10
Line 11
Line 12
Line 13
Line 14
Line 15
Line 16
Line 17
Line 18
Line 19
Line 20`
	b := `Line 1
Line 3
Line 4
Line 5
Line 6 has been changed
Line 7
Line 8
Line 9
Line 10
Line 11
Line 12
Line 13 has been changed
Line 14
Line 15
Line 16 has been changed
Line 17
Line 18
Line 19
Line 20
`
	expected := `--- filename
+++ filename
@@ -1,20 +1,19 @@
 Line 1
-Line 2
 Line 3
 Line 4
 Line 5
-Line 6
+Line 6 has been changed
 Line 7
 Line 8
 Line 9
 Line 10
 Line 11
 Line 12
-Line 13
+Line 13 has been changed
 Line 14
 Line 15
-Line 16
+Line 16 has been changed
 Line 17
 Line 18
 Line 19
-Line 20
\ No newline at end of file
+Line 20
`
	testUnifiedDiff(a, b, expected, t)
}

func TestUnifiedDiffTwoHunks(t *testing.T) {
	a := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7
Line 8
Line 9
Line 10
Line 11
Line 12
Line 13
Line 14
Line 15
Line 16
Line 17
Line 18
Line 19
Line 20
`
	b := `Line 1
Line 3
Line 4
Line 5
Line 6 has been changed
Line 7
Line 8
Line 9
Line 10
Line 11
Line 12
Line 13
Line 14 has been changed
Line 15
Line 16 has been changed
Line 17
Line 18
Line 19
Line 20
`
	expected := `--- filename
+++ filename
@@ -1,9 +1,8 @@
 Line 1
-Line 2
 Line 3
 Line 4
 Line 5
-Line 6
+Line 6 has been changed
 Line 7
 Line 8
 Line 9
@@ -11,9 +10,9 @@
 Line 11
 Line 12
 Line 13
-Line 14
+Line 14 has been changed
 Line 15
-Line 16
+Line 16 has been changed
 Line 17
 Line 18
 Line 19
`
	testUnifiedDiff(a, b, expected, t)
}
