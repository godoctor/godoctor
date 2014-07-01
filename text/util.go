// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines miscellaneous structs and utility methods that are used
// throughout the system.

package text

import (
	"fmt"
)

// An OffsetLength consists of two integers: a 0-based byte offset and a
// nonnegative length.  An OffsetLength is used to specify a region of a string
// or file.  For example, given the string "ABCDEFG", the substring CDE could
// be specified by OffsetLength{offset: 2, length: 3}.
type OffsetLength struct {
	// Byte offset of the first character (0-based)
	Offset int `json:"offset"`
	// Length in bytes (nonnegative)
	Length int `json:"length"`
}

// OffsetPastEnd returns the offset of the first byte immediately beyond the
// end of this region.  For example, a region at offset 2 with length 3
// occupies bytes 2 through 4, so this method would return 5.
func (o *OffsetLength) OffsetPastEnd() int {
	return o.Offset + o.Length
}

// Intersect returns the intersection (i.e., the overlapping region) of two
// intervals, or nil iff the intervals do not overlap.  A length-zero overlap
// is returned only if the two intervals are not adjacent.
func (o *OffsetLength) Intersect(other *OffsetLength) *OffsetLength {
	start := max(o.Offset, other.Offset)
	end := min(o.OffsetPastEnd(), other.OffsetPastEnd())
	len := end - start
	if len < 0 {
		return nil
	}
	if len == 0 && o.IsAdjacentTo(other) {
		return nil
	}
	return &OffsetLength{start, len}
}

// IsAdjacentTo returns true iff two intervals describe regions immediately
// next to one another, such as (offset 2, length 3) and (offset 5, length 1).
// Specifically, [a,b) is adjacent to [c,d) iff b == c or d == a.  Note that a
// length-zero interval is adjacent to itself.
func (o *OffsetLength) IsAdjacentTo(other *OffsetLength) bool {
	return o.OffsetPastEnd() == other.Offset ||
		other.OffsetPastEnd() == o.Offset
}

func (o *OffsetLength) String() string {
	return fmt.Sprintf("offset %d, length %d", o.Offset, o.Length)
}

// A TextSelection represents a selection in a text editor.  It consists of a
// filename, the line/column where the selected text begins, and the
// line/column where the text selection ends.  The end line and column must be
// greater than or equal to the start line and column, respectively.  Line and
// column numbers are 1-based.
type TextSelection struct {
	Filename  string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

func (s *TextSelection) String() string {
	return fmt.Sprintf("%s:%d,%d:%d,%d",
		s.Filename, s.StartLine, s.StartCol, s.EndLine, s.EndCol)
}

func (s *TextSelection) ShortString() string {
	return fmt.Sprintf("%d,%d:%d,%d",
		s.StartLine, s.StartCol, s.EndLine, s.EndCol)
}
