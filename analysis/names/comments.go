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
// occurrences of the given name (as a word, not a subword) and returns their
// source locations.  Position information is obtained from the given FileSet.
func FindInComments(name string, f *ast.File, fset *token.FileSet) []text.Extent {
	// FIXME: Where did the KMP stuff go?  Are the Go identifier rules
	// safe for substition in a regex (below)?  -JO
	result := []text.Extent{}
	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			slashIdx := fset.Position(comment.Slash).Offset
			whitespaceIdx := 1
			regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/\\*%s[\\PL]|[\\PL]%s$", name, name, name, name)
			re := regexp.MustCompile(regexpstring)
			matchcount := strings.Count(comment.Text, name)
			for _, idx := range re.FindAllStringIndex(comment.Text, matchcount) {
				var offset int
				if idx[0] == 0 {
					offset = slashIdx + idx[0] + whitespaceIdx + 1
				} else {
					offset = slashIdx + idx[0] + whitespaceIdx
				}
				result = append(result, text.Extent{offset, len(name)})
			}
		}

	}
	return result
}
