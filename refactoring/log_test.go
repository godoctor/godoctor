// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"testing"

	"golang-refactoring.org/go-doctor/text"
)

func TestLogEntry(t *testing.T) {
	e := LogEntry{false, INFO, "Message", "", text.Extent{}}
	assertEquals("Message", e.String(), t)
	e = LogEntry{false, WARNING, "Message", "", text.Extent{}}
	assertEquals("Warning: Message", e.String(), t)
	e = LogEntry{false, ERROR, "Message", "", text.Extent{}}
	assertEquals("Error: Message", e.String(), t)
	e = LogEntry{false, FATAL_ERROR, "Message", "", text.Extent{}}
	assertEquals("ERROR: Message", e.String(), t)

	e = LogEntry{false, WARNING, "Msg", "fn", text.Extent{1, 2}}
	assertEquals("Warning: fn, offset 1, length 2: Msg", e.String(), t)
}

func TestLog(t *testing.T) {
	var log *Log = NewLog()
	log.Log(WARNING, "A warning")
	log.Log(ERROR, "An error")
	var expected string = "Warning: A warning\nError: An error\n"
	assertEquals(expected, log.String(), t)
	log.Log(INFO, "Information")
	log.Log(FATAL_ERROR, "A fatal error")
	expected += "Information\nERROR: A fatal error\n"
	assertEquals(expected, log.String(), t)
}

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
// TODO(jeff): Copied from util_test.go
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		t.Fatalf("Expected: %s Actual: %s", expected, actual)
	}
}
