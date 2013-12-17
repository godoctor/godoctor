// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines miscellaneous structs and utility methods that are used
// throughout the system.

// Contributors: Jeff Overbey

package doctor

import (
	"fmt"
	"strconv"
)

// An OffsetLength consists of two integers: a 0-based offset and a nonnegative
// length.  An OffsetLength is used to specify a region of a string or file.
// For example, given the string "ABCDEFG", the substring CDE could be
// specified by
//     OffsetLength{offset: 2, length: 3}
type OffsetLength struct {
	Offset int `json:"offset"`
	Length int `json:"length"`
}

func (o *OffsetLength) OffsetPastEnd() int {
	return o.Offset + o.Length
}

func (o *OffsetLength) String() string {
	return "offset " + strconv.Itoa(o.Offset) +
		", length " + strconv.Itoa(o.Length)
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
