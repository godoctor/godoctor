// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

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
