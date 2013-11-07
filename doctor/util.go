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
	offset int
	length int
}

func (o *OffsetLength) String() string {
	return "offset " + strconv.Itoa(o.offset) +
		", length " + strconv.Itoa(o.length)
}

// A TextSelection represents a selection in a text editor.  It consists of a
// filename, the line/column where the selected text begins, and the
// line/column where the text selection ends.  The end line and column must be
// greater than or equal to the start line and column, respectively.  Line and
// column numbers are 1-based.
type TextSelection struct {
	filename  string
	startLine int
	startCol  int
	endLine   int
	endCol    int
}

func (s *TextSelection) String() string {
	return fmt.Sprintf("%s:%d,%d:%d,%d",
		s.filename, s.startLine, s.startCol, s.endLine, s.endCol)
}
