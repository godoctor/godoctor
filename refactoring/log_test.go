// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package refactoring

import (
	"testing"

	"golang-refactoring.org/go-doctor/text"
)

func TestEntry(t *testing.T) {
	e := Entry{false, Info, "Message", "", &text.Extent{}}
	assertEquals("Message", e.String(), t)
	e = Entry{false, Warning, "Message", "", &text.Extent{}}
	assertEquals("Warning: Message", e.String(), t)
	e = Entry{false, Error, "Message", "", &text.Extent{}}
	assertEquals("Error: Message", e.String(), t)

	e = Entry{false, Warning, "Msg", "fn", &text.Extent{1, 2}}
	assertEquals("Warning: fn, offset 1, length 2: Msg", e.String(), t)
}

func TestLog(t *testing.T) {
	var log *Log = NewLog()
	log.Info("Info")
	log.Warn("A warning")
	log.Error("An error")
	var expected string = "Info\nWarning: A warning\nError: An error\n"
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
