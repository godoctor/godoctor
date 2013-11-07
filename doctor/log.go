// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Log struct and associated methods.  Every refactoring
// returns a Log, which contains informational messages, warnings, and errors
// generated during the refactoring process.  If the log is nonempty, it should
// be displayed to the user before a refactoring's changes are applied.

// Contributors: Jeff Overbey

package doctor

import (
	"bytes"
)

// Every LogEntry has a severity: INFO, WARNING, ERROR, or FATAL_ERROR.  An
// ERROR (non-fatal) indicates that the refactoring may not preserve behavior,
// but the transformation can still be applied at the user's risk.  In contrast,
// a FATAL_ERROR indicates that the refactoring cannot continue because it is
// impossible to construct or apply the transformation (e.g., the selection is
// invalid, the input file cannot be parsed, etc.)
type Severity int

const (
	INFO Severity = iota
	WARNING
	ERROR
	FATAL_ERROR
)

// A LogEntry constitutes a single entry in a Log.  Every LogEntry has a
// severity and a message.  If the filename is a nonempty string, the LogEntry
// is associated with a particular position in the given file.  Some log
// entries are marked as "initial."  These indicate semantic errors that were
// present in the input file (e.g., unresolved identifiers, unnecessary
// imports, etc.) before the refactoring was started.
type LogEntry struct {
	isInitial bool
	severity  Severity
	message   string
	filename  string
	position  OffsetLength
}

// A Log is used to store informational messages, warnings, and errors that
// will be presented to the user before a refactoring's changes are applied.
type Log struct {
	entries []LogEntry
}

func (entry *LogEntry) String() string {
	var buffer bytes.Buffer
	switch entry.severity {
	case INFO:
		// No prefix
	case WARNING:
		buffer.WriteString("Warning: ")
	case ERROR:
		buffer.WriteString("Error: ")
	case FATAL_ERROR:
		buffer.WriteString("ERROR: ")
	}
	if entry.filename != "" {
		buffer.WriteString(entry.filename)
		buffer.WriteString(", ")
		buffer.WriteString(entry.position.String())
		buffer.WriteString(": ")
	}
	buffer.WriteString(entry.message)
	return buffer.String()
}

// NewLog returns a new, empty Log.
func NewLog() *Log {
	log := new(Log)
	log.entries = []LogEntry{}
	return log
}

// Clear removes all entries from the log.
func (log *Log) Clear() {
	log.entries = []LogEntry{}
}

// LogInitial adds a message to the given log with the given severity, and
// marks the entry as an initial error.  Initial errors are semantic errors
// that are present in the file before refactoring starts; some refactorings
// work in the presence of errors, and others may not.  The message is not
// associated with any particular file.
func (log *Log) LogInitial(severity Severity, message string,
	filename string, offset int, length int) {
	log.entries = append(log.entries, LogEntry{
		isInitial: true,
		severity:  severity,
		message:   message,
		filename:  filename,
		position:  OffsetLength{offset, length}})
}

// Log adds a message to the given log with the given severity.  The message
// is not associated with any particular file.
func (log *Log) Log(severity Severity, message string) {
	log.entries = append(log.entries, LogEntry{
		isInitial: false,
		severity:  severity,
		message:   message,
		filename:  "",
		position:  OffsetLength{0, 0}})
}

func (log *Log) String() string {
	var buffer bytes.Buffer
	for _, entry := range log.entries {
		buffer.WriteString(entry.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

// ContainsNonInitialErrors returns true if the log contains at least one
// non-initial error or fatal error.
func (log *Log) ContainsNonInitialErrors() bool {
	return log.contains(func(entry LogEntry) bool {
		return entry.severity >= ERROR && !entry.isInitial
	})
}

// ContainsNonInitialErrors returns true if the log contains at least one
// initial error or fatal error.
func (log *Log) ContainsInitialErrors() bool {
	return log.contains(func(entry LogEntry) bool {
		return entry.severity >= ERROR && entry.isInitial
	})
}

// ContainsNonInitialErrors returns true if the log contains at least one
// error or fatal error.  The error may be an initial error, or it may not.
func (log *Log) ContainsErrors() bool {
	return log.contains(func(entry LogEntry) bool {
		return entry.severity >= ERROR
	})
}

func (log *Log) contains(predicate func(LogEntry) bool) bool {
	for _, entry := range log.entries {
		if predicate(entry) {
			return true
		}
	}
	return false
}

// RemoveInitialEntries removes any initial entries from the log.  Entries that
// are not marked as initial are retained.
func (log *Log) RemoveInitialEntries() {
	newEntries := []LogEntry{}
	for _, entry := range log.entries {
		if !entry.isInitial {
			newEntries = append(newEntries, entry)
		}
	}
	log.entries = newEntries
}

// ChangeInitialErrorsToWarnings changes the severity of any initial errors and
// fatal errors in the log to WARNING severity.
func (log *Log) ChangeInitialErrorsToWarnings() {
	newEntries := []LogEntry{}
	for _, entry := range log.entries {
		if entry.isInitial {
			entry.severity = WARNING
			newEntries = append(newEntries, entry)
		} else {
			newEntries = append(newEntries, entry)
		}
	}
	log.entries = newEntries
}
