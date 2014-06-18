// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

const diffTestDir = "../testdata/diff/"

func TestDiffs(t *testing.T) {
	strings := []string{"", "ABCABBA", "CBABAC"}
	for _, a := range strings {
		for _, b := range strings {
			testDiffs(a, b, t)
		}
	}
	for _, b := range []string{"a\nbcd", "abcfg", "defg", "abcd", "ag",
		"bcd", "abd", "efg", "axy", "xcg", "xcdghy", "xabcdefgy"} {
		testDiffs("abcdefg", b, t)
	}
}

func testDiffs(a, b string, t *testing.T) {
	diff := Diff(strings.Split(a, ""), strings.Split(b, ""))
	result, err := ApplyToString(diff, a)
	failIfError(err, t)
	assertEquals(b, result, t)
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
	assertTrue(len(r.leadingCtxLines) == numCtxLines, t)
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

func TestUnifiedDiff(t *testing.T) {
	testDirs, err := ioutil.ReadDir(diffTestDir)
	failIfError(err, t)
	for _, testDirInfo := range testDirs {
		if testDirInfo.IsDir() {
			fmt.Printf("Diff Test %s\n", testDirInfo.Name())
			dir := filepath.Join(diffTestDir, testDirInfo.Name())
			from := readFile(filepath.Join(dir, "from.txt"), t)
			to := readFile(filepath.Join(dir, "to.txt"), t)
			diff := readFile(filepath.Join(dir, "diff.txt"), t)
			testUnifiedDiff(from, to, diff, testDirInfo.Name(), t)
		}
	}
}

func readFile(path string, t *testing.T) string {
	bytes, err := ioutil.ReadFile(path)
	failIfError(err, t)
	return string(bytes)
}

func testUnifiedDiff(a, b, expected, name string, t *testing.T) {
	edits := Diff(strings.SplitAfter(a, "\n"), strings.SplitAfter(b, "\n"))
	s, _ := ApplyToString(edits, a)
	assertEquals(b, s, t)
	patch, _ := edits.CreatePatch(strings.NewReader(a))
	if patch.String() != expected {
		t.Fatalf("Diff test %s failed.  Expected:\n[%s]\nActual:\n[%s]\n",
			name, expected, patch.String())
	}
}
