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

func digraphClosure(digraph [][]int) [][]int {
	if !validateGraph(digraph) {
		panic("invalid graph")
	}
	result := make([][]int, len(digraph))
	index := make([]int, len(digraph))
	lowlink := make([]int, len(digraph))
	stack := []int{}
	nextIndex := 1
	var scc func(v int)
	scc = func(v int) {
		result[v] = []int{v}
		index[v] = nextIndex
		lowlink[v] = nextIndex
		nextIndex++
		stack = append(stack, v)
		for _, w := range digraph[v] {
			if index[w] == 0 {
				scc(w)
				lowlink[v] = min(lowlink[v], lowlink[w])
			} else if contains(stack, w) {
				lowlink[v] = min(lowlink[v], lowlink[w])
			}
			result[v] = union(result[v], result[w])
		}
		if lowlink[v] == index[v] {
			for {
				w := stack[len(stack)-1]
				stack = stack[0 : len(stack)-1]
				result[w] = result[v]
				if w == v {
					break
				}
			}
		}
	}
	for i, _ := range digraph {
		if index[i] == 0 {
			scc(i)
		}
	}
	return result
}

func validateGraph(digraph [][]int) bool {
	for _, adj := range digraph {
		for _, idx := range adj {
			if idx < 0 || idx >= len(digraph) {
				return false
			}
		}
	}
	return true
}

func union(u, v []int) []int {
	result := make([]int, len(u), len(u)+len(v))
	copy(result, u)
	for _, value := range v {
		if !contains(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func contains(slice []int, value int) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
