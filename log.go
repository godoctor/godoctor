package doctor

// This file defines the Log struct and associated methods.

import (
	"bytes"
)

// Every LogEntry has a severity: INFO, WARNING, ERROR, or FATAL_ERROR
type Severity int

const (
	INFO Severity = iota
	WARNING
	ERROR
	FATAL_ERROR
)

// A LogEntry constitutes a single entry in a Log.  Every LogEntry has a
// severity and a message.  If the filename is a nonempty string, the LogEntry
// is associated with a particular position in the given file.
type LogEntry struct {
	severity Severity
	message  string
	filename string
	position OffsetLength
}

// A Log is used to store informational messages, warnings, and errors that
// will be presented to the user.
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

// Clear removes all entries from the error log.
func (log *Log) Clear() {
	log.entries = []LogEntry{}
}

// Log adds a message to the given log with the given severity.  The message
// is not associated with any particular file.
func (log *Log) Log(severity Severity, message string) {
	log.entries = append(log.entries, LogEntry{
		severity: severity,
		message:  message,
		filename: "",
		position: OffsetLength{0, 0}})
}

func (log *Log) String() string {
	var buffer bytes.Buffer
	for _, entry := range log.entries {
		buffer.WriteString(entry.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func (log *Log) ContainsErrors() bool {
	for _, entry := range log.entries {
		if entry.severity >= ERROR {
			return true
		}
	}
	return false
}
