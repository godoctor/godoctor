// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package text

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtent(t *testing.T) {
	ol := Extent{Offset: 5, Length: 20}
	assertEquals("offset 5, length 20", ol.String(), t)
}

func TestExtentIntersect(t *testing.T) {
	ol15 := &Extent{Offset: 1, Length: 5}
	ol30 := &Extent{Offset: 3, Length: 0}
	ol33 := &Extent{Offset: 3, Length: 3}
	ol51 := &Extent{Offset: 5, Length: 1}
	ol61 := &Extent{Offset: 6, Length: 1}

	type test struct {
		ol1, ol2 *Extent
		expect   string
	}

	tests := []test{
		test{ol15, ol15, "offset 1, length 5"},
		test{ol15, ol30, "offset 3, length 0"},
		test{ol15, ol33, "offset 3, length 3"},
		test{ol15, ol51, "offset 5, length 1"},
		test{ol15, ol61, ""},

		test{ol30, ol15, "offset 3, length 0"},
		test{ol30, ol30, ""},
		test{ol30, ol33, ""},
		test{ol30, ol51, ""},
		test{ol30, ol61, ""},

		test{ol33, ol15, "offset 3, length 3"},
		test{ol33, ol30, ""},
		test{ol33, ol33, "offset 3, length 3"},
		test{ol33, ol51, "offset 5, length 1"},
		test{ol33, ol61, ""},

		test{ol51, ol15, "offset 5, length 1"},
		test{ol51, ol30, ""},
		test{ol51, ol33, "offset 5, length 1"},
		test{ol51, ol51, "offset 5, length 1"},
		test{ol51, ol61, ""},

		test{ol61, ol15, ""},
		test{ol61, ol30, ""},
		test{ol61, ol33, ""},
		test{ol61, ol51, ""},
		test{ol61, ol61, "offset 6, length 1"},
	}

	for _, tst := range tests {
		overlap := tst.ol1.Intersect(tst.ol2)
		if overlap == nil && tst.expect != "" {
			t.Fatalf("%s ∩ %s produced nil, expected %s",
				tst.ol1, tst.ol2, tst.expect)
		} else if overlap != nil && tst.expect != overlap.String() {
			t.Fatalf("%s ∩ %s produced %s, expected %s",
				tst.ol1, tst.ol2, overlap, tst.expect)
		}
	}
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
