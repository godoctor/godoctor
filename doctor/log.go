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
	Severity Severity     `json:"severity"`
	Message  string       `json:"message"`
	Filename string       `json:"filename"`
	Position OffsetLength `json:"position"`
}

// A Log is used to store informational messages, warnings, and errors that
// will be presented to the user.
type Log struct {
	Entries []LogEntry `json:"entries"`
}

func (entry *LogEntry) String() string {
	var buffer bytes.Buffer
	switch entry.Severity {
	case INFO:
		// No prefix
	case WARNING:
		buffer.WriteString("Warning: ")
	case ERROR:
		buffer.WriteString("Error: ")
	case FATAL_ERROR:
		buffer.WriteString("ERROR: ")
	}
	if entry.Filename != "" {
		buffer.WriteString(entry.Filename)
		buffer.WriteString(", ")
		buffer.WriteString(entry.Position.String())
		buffer.WriteString(": ")
	}
	buffer.WriteString(entry.Message)
	return buffer.String()
}

// NewLog returns a new, empty Log.
func NewLog() *Log {
	log := new(Log)
	log.Entries = []LogEntry{}
	return log
}

// Clear removes all Entries from the error log.
func (log *Log) Clear() {
	log.Entries = []LogEntry{}
}

// Log adds a message to the given log with the given severity.  The message
// is not associated with any particular file.
func (log *Log) Log(severity Severity, message string) {
	log.Entries = append(log.Entries, LogEntry{
		Severity: severity,
		Message:  message,
		Filename: "",
		Position: OffsetLength{0, 0}})
}

func (log *Log) String() string {
	var buffer bytes.Buffer
	for _, entry := range log.Entries {
		buffer.WriteString(entry.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func (log *Log) ContainsErrors() bool {
	for _, entry := range log.Entries {
		if entry.Severity >= ERROR {
			return true
		}
	}
	return false
}
