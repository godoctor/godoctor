// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"golang-refactoring.org/go-doctor/text"
)

// FindInComments searches the comments of the given packages' source files for
// occurrences of the given name (as a word) and returns their source locations.
func FindInComments(name string, f *ast.File, fset *token.FileSet) []text.Extent {
	result := []text.Extent{}
	for _, comment := range f.Comments {
		if strings.Contains(comment.List[0].Text, name) {
			result = append(result, occurrences(name, comment, fset)...)
		}
	}
	return result
}

// occurrences returns the source location of the given name in the given
// CommentGroup.
func occurrences(name string, comment *ast.CommentGroup, fset *token.FileSet) []text.Extent {
	var result []text.Extent
	whitespaceindex := 1
	regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/\\*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	re := regexp.MustCompile(regexpstring)
	matchcount := strings.Count(comment.List[0].Text, name)
	for _, matchindex := range re.FindAllStringIndex(comment.List[0].Text, matchcount) {
		var offset int
		if matchindex[0] == 0 {
			offset = fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex + 1
		} else {
			offset = fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex
		}
		result = append(result, text.Extent{offset, len(name)})
	}
	return result
}
