// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOffsetLength(t *testing.T) {
	ol := OffsetLength{Offset: 5, Length: 20}
	assertEquals("offset 5, length 20", ol.String(), t)
}

func TestUnionContains(t *testing.T) {
	set1 := []int{1, 2, 3}
	//	set2 := []int{3, 1, 4}
	//	set3 := []int{5}
	//	set4 := []int{}
	//	assertEquals([]int{1, 2, 3, 4}, union(set1, set2))
	//	assertEquals([]int{1, 2, 3, 5}, union(set1, set3))
	//	assertEquals(set3, union(set3, set4))
	assertFalse(contains(set1, 0), t)
	assertTrue(contains(set1, 1), t)
	assertTrue(contains(set1, 2), t)
	assertTrue(contains(set1, 3), t)
	assertFalse(contains(set1, 4), t)
}

func TestGraphClosure(t *testing.T) {
	// 0 --> 1
	// |     ^
	// v     |  _
	// 3 <-- 2 <_|
	// |
	// v
	// 4
	// |
	// v
	// 5
	graph1 := [][]int{
		[]int{1, 3},
		[]int{},
		[]int{1, 2, 3},
		[]int{4},
		[]int{5},
		[]int{}}
	expected := "[[0 1 3 4 5] [1] [2 1 3 4 5] [3 4 5] [4 5] [5]]"
	actual := fmt.Sprintf("%v", digraphClosure(graph1))
	assertEquals(expected, actual, t)
}

// -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// These are utility methods used by other tests as well.  They need to be in
// a file named something_test.go so that command line arguments used for
// testing do not get compiled into the main driver (TODO maybe there's another
// way around that?), and this seemed like a reasonable place for them...

func fatalf(t *testing.T, format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(2)
	if ok {
		var msg string
		if len(args) == 0 {
			msg = format
		} else {
			msg = fmt.Sprintf(format, args...)
		}
		t.Fatalf("from %s:%d: %s", filepath.Base(file), line, msg)
	}
}

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		fatalf(t, "Expected: %s Actual: %s", expected, actual)
	}
}

// assertError is a utility method for unit tests that marks a function as
// having failed if the given string does not begin with "ERROR: "
func assertError(result string, t *testing.T) {
	if !strings.HasPrefix(result, "ERROR: ") {
		fatalf(t, "Expected error; actual: \"%s\"", result)
	}
}

// assertTrue is a utility method for unit tests that marks a function as
// having succeeded iff the supplied value is true
func assertTrue(value bool, t *testing.T) {
	if value != true {
		fatalf(t, "assertTrue failed")
	}
}

// assertFalse is a utility method for unit tests that marks a function as
// having succeeded iff the supplied value is true
func assertFalse(value bool, t *testing.T) {
	if value != false {
		fatalf(t, "assertFalse failed")
	}
}
