// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines text.Extent and text.Selection, which describe regions
// within a particular text file.

package text

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"code.google.com/p/go.tools/go/loader"
)

// An Extent consists of two integers: a 0-based byte offset and a
// nonnegative length.  An Extent is used to specify a region of a string
// or file.  For example, given the string "ABCDEFG", the substring CDE could
// be specified by Extent{offset: 2, length: 3}.
type Extent struct {
	// Byte offset of the first character (0-based)
	Offset int `json:"offset"`
	// Length in bytes (nonnegative)
	Length int `json:"length"`
}

// OffsetPastEnd returns the offset of the first byte immediately beyond the
// end of this region.  For example, a region at offset 2 with length 3
// occupies bytes 2 through 4, so this method would return 5.
func (o *Extent) OffsetPastEnd() int {
	return o.Offset + o.Length
}

// Intersect returns the intersection (i.e., the overlapping region) of two
// intervals, or nil iff the intervals do not overlap.  A length-zero overlap
// is returned only if the two intervals are not adjacent.
func (o *Extent) Intersect(other *Extent) *Extent {
	start := max(o.Offset, other.Offset)
	end := min(o.OffsetPastEnd(), other.OffsetPastEnd())
	len := end - start
	if len < 0 {
		return nil
	}
	if len == 0 && o.IsAdjacentTo(other) {
		return nil
	}
	return &Extent{start, len}
}

// IsAdjacentTo returns true iff two intervals describe regions immediately
// next to one another, such as (offset 2, length 3) and (offset 5, length 1).
// Specifically, [a,b) is adjacent to [c,d) iff b == c or d == a.  Note that a
// length-zero interval is adjacent to itself.
func (o *Extent) IsAdjacentTo(other *Extent) bool {
	return o.OffsetPastEnd() == other.Offset ||
		other.OffsetPastEnd() == o.Offset
}

func (o *Extent) String() string {
	return fmt.Sprintf("offset %d, length %d", o.Offset, o.Length)
}

// A Selection represents a selection in a text editor.  It consists of a
// filename, the line/column where the selected text begins, and the
// line/column where the text selection ends.  The end line and column must be
// greater than or equal to the start line and column, respectively.  Line and
// column numbers are 1-based.
type Selection interface {
	Convert(*loader.Program) (*ast.File, token.Pos, token.Pos)
	AbsFilename() string
	PosString() string
	String() string
}

type LineColSelection struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

func (lc *LineColSelection) Convert(program *loader.Program) (*ast.File, token.Pos, token.Pos) {
	absFilename, _ := filepath.Abs(lc.Filename)
	var file *ast.File
	for _, pkgInfo := range program.AllPackages {
		for _, f := range pkgInfo.Files {
			thisFile := program.Fset.Position(f.Pos()).Filename
			if thisFile == lc.Filename || thisFile == absFilename {
				file = f
			}
		}
	}
	startPos := lineColToPos(program, file, lc.StartLine, lc.StartCol)
	endPos := lineColToPos(program, file, lc.EndLine, lc.EndCol)
	return file, startPos, endPos
}

func (lc *LineColSelection) AbsFilename() string {
	return lc.Filename
}

// TODO add piece that conditionally checks if offset/length or row/col
// Returns a new Selection type that will either be LineColSelection
// or OffsetLengthSelection
func NewSelection(filename string, pos string) (Selection, error) {
	absFilename, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("invalid filename")
	}

	args := strings.Split(pos, ":")

	if len(args) < 2 {
		return nil, fmt.Errorf("invalid -pos")
	}

	sl, sc := parseLineCol(args[0])
	el, ec := parseLineCol(args[1])

	if sl < 0 || sc < 0 || el < 0 || ec < 0 {
		return nil, fmt.Errorf("invalid -pos line, col")
	}

	return &LineColSelection{Filename: absFilename, StartLine: sl, StartCol: sc,
		EndLine: el, EndCol: ec}, nil
}

func (lc *LineColSelection) PosString() string {
	return fmt.Sprintf("%d,%d:%d,%d",
		lc.StartLine, lc.StartCol, lc.EndLine, lc.EndCol)
}

func (lc *LineColSelection) String() string {
	return fmt.Sprintf("%s: %s", lc.Filename, lc.PosString())
}

// lineColToPos converts a line/column position (where the first character in a
// file is at // line 1, column 1) into a token.Pos
func lineColToPos(program *loader.Program, file *ast.File, line int, column int) token.Pos {
	if file == nil {
		panic("file is nil")
	}
	lastLine := -1
	thisColumn := 1
	tfile := program.Fset.File(file.Package)
	for i, size := 0, tfile.Size(); i < size; i++ {
		pos := tfile.Pos(i)
		thisLine := tfile.Line(pos)
		if thisLine != lastLine {
			thisColumn = 1
		} else {
			thisColumn++
		}
		if thisLine == line && thisColumn == column {
			return pos
		}
		lastLine = thisLine
	}
	return file.Pos()
}

// e.g. 302,6
func parseLineCol(linecol string) (int, int) {
	lc := strings.Split(linecol, ",")
	if l, err := strconv.ParseInt(lc[0], 10, 32); err == nil {
		if c, err := strconv.ParseInt(lc[1], 10, 32); err == nil {
			return int(l), int(c)
		}
	}

	return -1, -1
}
