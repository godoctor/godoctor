// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

package doctor

import (
	"testing"
)

func TestLogEntry(t *testing.T) {
	e := LogEntry{false, INFO, "Message", "", OffsetLength{}}
	assertEquals("Message", e.String(), t)
	e = LogEntry{false, WARNING, "Message", "", OffsetLength{}}
	assertEquals("Warning: Message", e.String(), t)
	e = LogEntry{false, ERROR, "Message", "", OffsetLength{}}
	assertEquals("Error: Message", e.String(), t)
	e = LogEntry{false, FATAL_ERROR, "Message", "", OffsetLength{}}
	assertEquals("ERROR: Message", e.String(), t)

	e = LogEntry{false, WARNING, "Msg", "fn", OffsetLength{1, 2}}
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
