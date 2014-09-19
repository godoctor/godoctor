// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package names

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"code.google.com/p/go.tools/go/loader"
	"golang-refactoring.org/go-doctor/text"
)

// FindInComments searches the comments of the given packages' source files for
// occurrences of the given name (as a word) and returns their source locations.
func (r *Finder) FindInComments(name string, pkgs map[*loader.PackageInfo]bool, result map[string][]text.Extent) map[string][]text.Extent {
	for pkgInfo := range pkgs {
		for _, f := range pkgInfo.Files {
			fname := r.program.Fset.Position(f.Pos()).Filename
			for _, comment := range f.Comments {
				if strings.Contains(comment.List[0].Text, name) {
					result = r.addOccurrences(fname, comment, name, result)
				}
			}
		}
	}
	return result
}

// findInFileComments returns the source location of  selected identifier names in
// comments, appends them to the already found source locations of
// selected identifier objects (result), and returns the result.
func (r *Finder) addOccurrences(filename string, comment *ast.CommentGroup, name string, result map[string][]text.Extent) map[string][]text.Extent {
	var whitespaceindex int = 1
	var offset int
	//regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	regexpstring := fmt.Sprintf("[\\PL]%s[\\PL]|//%s[\\PL]|/\\*%s[\\PL]|[\\PL]%s$", name, name, name, name)
	re := regexp.MustCompile(regexpstring)
	matchcount := strings.Count(comment.List[0].Text, name)
	for _, matchindex := range re.FindAllStringIndex(comment.List[0].Text, matchcount) {
		if matchindex[0] == 0 {
			offset = r.program.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex + 1
		} else {
			offset = r.program.Fset.Position(comment.List[0].Slash).Offset + matchindex[0] + whitespaceindex
		}
		result[filename] = append(result[filename], text.Extent{offset, len(name)})
	}
	return result
}
