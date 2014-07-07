// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines text.Extent and text.Selection, which describe regions
// within a particular text file.

package text

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// A Selection represents a range of text within a particular file.  It is
// used to represent a selection in a text editor.
type Selection interface {
	// Convert returns start and end positions corresponding to this
	// selection.  It returns an error if this selection corresponds to a
	// file that is not in the given FileSet, or if the selected region is
	// not in range.
	Convert(*token.FileSet) (token.Pos, token.Pos, error)
	// GetFilename returns the file containing this selection.  The
	// returned filename may be an absolute or relative path and does is
	// not guaranteed to correspond to a valid file.
	GetFilename() string
	// String returns a human-readable representation of this Selection.
	String() string
}

// A LineColSelection is a Selection consisting of a filename, the line/column
// where the selected text begins, and the line/column where the text selection
// ends.  The end line and column must be greater than or equal to the start
// line and column, respectively.  Line and column numbers are 1-based.
type LineColSelection struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

func (lc *LineColSelection) Convert(fset *token.FileSet) (token.Pos, token.Pos, error) {
	file := findFile(fset, lc.Filename)
	if file == nil {
		// error message from findQueryPos in go.tools/oracle/pos.go
		return 0, 0, fmt.Errorf("couldn't find file containing position")
	}

	startPos, err := lineColToPos(file, lc.StartLine, lc.StartCol)
	if err != nil {
		return 0, 0, err
	}

	endPos, err := lineColToPos(file, lc.EndLine, lc.EndCol)
	if err != nil {
		return 0, 0, err
	}
	return startPos, endPos, nil
}

// findFile returns the file corresponding to the given filename, or nil if no
// file can be found with that filename.  The absolute path of the returned
// file can be found via f.Name().
func findFile(fset *token.FileSet, filename string) *token.File {
	// from findQueryPos in go.tools/oracle/pos.go
	var file *token.File
	fset.Iterate(func(f *token.File) bool {
		if sameFile(filename, f.Name()) {
			file = f
			return false // done
		}
		return true // continue
	})
	return file
}

// sameFile returns true if x and y have the same basename and denote
// the same file.
func sameFile(x, y string) bool { // from go.tools/oracle/pos.go
	if filepath.Base(x) == filepath.Base(y) { // (optimisation)
		if xi, err := os.Stat(x); err == nil {
			if yi, err := os.Stat(y); err == nil {
				return os.SameFile(xi, yi)
			}
		}
	}
	return false
}

// lineColToPos converts a line/column position to a token.Pos.  The first
// character in a file is considered to be at line 1, column 1.
func lineColToPos(file *token.File, line int, column int) (token.Pos, error) {
	// Binary search to find a position on the given line
	lastOffset := file.Size() - 1
	start := 0
	end := lastOffset
	mid := (start + end) / 2
	for start <= end {
		midLine := file.Line(file.Pos(mid))
		if line == midLine {
			break
		} else if line < midLine {
			end = mid - 1
		} else /* line > midLine */ {
			start = mid + 1
		}
		mid = (start + end) / 2
	}

	// Now mid is some position on the correct line; add/subtract to find
	// the position at the correct column
	difference := file.Position(file.Pos(mid)).Column - column
	pos := file.Pos(mid - difference)
	p := file.Position(pos)
	if p.Line != line || p.Column != column {
		return pos, fmt.Errorf("Invalid position: line %d, column %d",
			line, column)
	}
	return pos, nil
}

func (lc *LineColSelection) GetFilename() string {
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

func (lc *LineColSelection) String() string {
	return fmt.Sprintf("%s: %d,%d:%d,%d", lc.Filename,
		lc.StartLine, lc.StartCol, lc.EndLine, lc.EndCol)
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
