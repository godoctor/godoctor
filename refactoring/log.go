// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the Log struct and associated methods.  Every refactoring
// returns a Log, which contains informational messages, warnings, and errors
// generated during the refactoring process.  If the log is nonempty, it should
// be displayed to the user before a refactoring's changes are applied.
//
// TERMINOLOGY: "Initial entries" are those that are added when the program is
// first loaded, before the refactoring begins.  They are used to record
// semantic errors that are present file before refactoring starts.  Some
// refactorings work in the presence of errors, and others may not.  Therefore,
// there are two methods to modify initial entries: one that converts initial
// errors to warnings, and another that removes initial entries altogether.

package refactoring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"code.google.com/p/go.tools/go/loader"

	"go/ast"
	"go/token"

	"golang-refactoring.org/go-doctor/text"
)

// A Severity indicates whether a log entry describes an informational message,
// a warning, or an error.
type Severity int

const (
	Info    Severity = iota // informational message
	Warning                 // warning, something to be cautious of
	Error                   // the refactoring transformation is, or might be, invalid
)

// A Entry constitutes a single entry in a Log.  Every Entry has a
// severity and a message.  If the filename is a nonempty string, the Entry
// is associated with a particular position in the given file.  Some log
// entries are marked as "initial."  These indicate semantic errors that were
// present in the input file (e.g., unresolved identifiers, unnecessary
// imports, etc.) before the refactoring was started.
type Entry struct {
	isInitial bool
	Severity  Severity     `json:"severity"`
	Message   string       `json:"message"`
	Filename  string       `json:"filename"`
	Position  *text.Extent `json:"position"`
}

// A Log is used to store informational messages, warnings, and errors that
// will be presented to the user before a refactoring's changes are applied.
type Log struct {
	Entries []*Entry `json:"entries"`
}

func (entry *Entry) String() string {
	var buffer bytes.Buffer
	switch entry.Severity {
	case Info:
		// No prefix
	case Warning:
		buffer.WriteString("Warning: ")
	case Error:
		buffer.WriteString("Error: ")
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

// NewLog returns a new Log with no entries.
func NewLog() *Log {
	log := new(Log)
	log.Entries = []*Entry{}
	return log
}

// Clear removes all Entries from the error log.
func (log *Log) Clear() {
	log.Entries = []*Entry{}
}

// Infof adds an informational message (an entry with Info severity) to a log.
func (log *Log) Infof(format string, v ...interface{}) {
	log.log(Info, format, v...)
}

// Info adds an informational message (an entry with Info severity) to a log.
func (log *Log) Info(entry interface{}) {
	log.log(Info, "%v", entry)
}

// Warnf adds an entry with Warning severity to a log.
func (log *Log) Warnf(format string, v ...interface{}) {
	log.log(Warning, format, v...)
}

// Warn adds an entry with Warning severity to a log.
func (log *Log) Warn(entry interface{}) {
	log.log(Warning, "%v", entry)
}

// Errorf adds an entry with Error severity to a log.
func (log *Log) Errorf(format string, v ...interface{}) {
	log.log(Error, format, v...)
}

// Error adds an entry with Error severity to a log.
func (log *Log) Error(entry interface{}) {
	log.log(Error, "%v", entry)
}

func (log *Log) log(severity Severity, format string, v ...interface{}) {
	log.Entries = append(log.Entries, &Entry{
		isInitial: false,
		Severity:  severity,
		Message:   fmt.Sprintf(format, v...),
		Filename:  "",
		Position:  &text.Extent{0, 0}})
}

// Associate associates the most recently-logged entry with the given filename.
func (log *Log) Associate(filename string) {
	if len(log.Entries) == 0 {
		return
	}
	entry := log.Entries[len(log.Entries)-1]
	entry.Filename = displayablePath(filename)
}

// AssociatePos associates the most recently-logged entry with the file and
// offset denoted by the given Pos.
func (log *Log) AssociatePos(fset *token.FileSet, start, end token.Pos) {
	if len(log.Entries) == 0 {
		return
	}
	entry := log.Entries[len(log.Entries)-1]
	entry.Filename = displayablePath(fset.Position(start).Filename)
	entry.Position = &text.Extent{fset.Position(start).Offset, int(end - start)}
}

// AssociateNode associates the most recently-logged entry with the region of
// source code corresponding to the given AST Node.
func (log *Log) AssociateNode(p *loader.Program, node ast.Node) {
	log.AssociatePos(p.Fset, node.Pos(), node.End())
}

// displayablePath returns a path for the given file relative to the current
// directory, if possible, and the original filename otherwise.  It is intended
// for use in error messages.
func displayablePath(file string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return file
	}

	absPath, err := filepath.Abs(file)
	if err != nil {
		return file
	}

	relativePath, err := filepath.Rel(cwd, absPath)
	if err != nil || relativePath == "" {
		return file
	}

	return relativePath
}

// MarkInitial marks all entries that have been logged so far as initial
// entries.  Subsequent entries will not be marked as initial unless this
// method is called again at a later point in time.
func (log *Log) MarkInitial() {
	for _, entry := range log.Entries {
		entry.isInitial = true
	}
}

func (log *Log) String() string {
	var buffer bytes.Buffer
	for _, entry := range log.Entries {
		buffer.WriteString(entry.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

// ContainsErrors returns true if the log contains at least one error.  The
// error may be an initial entry, or it may not.
func (log *Log) ContainsErrors() bool {
	return log.contains(func(entry *Entry) bool {
		return entry.Severity >= Error
	})
}

func (log *Log) contains(predicate func(*Entry) bool) bool {
	for _, entry := range log.Entries {
		if predicate(entry) {
			return true
		}
	}
	return false
}

// RemoveInitialEntries removes all initial entries from the log.  Entries that
// are not marked as initial are retained.
func (log *Log) RemoveInitialEntries() {
	newEntries := []*Entry{}
	for _, entry := range log.Entries {
		if !entry.isInitial {
			newEntries = append(newEntries, entry)
		}
	}
	log.Entries = newEntries
}

// ChangeInitialErrorsToWarnings changes the severity of any initial errors to
// Warning severity.
func (log *Log) ChangeInitialErrorsToWarnings() {
	newEntries := []*Entry{}
	for _, entry := range log.Entries {
		if entry.isInitial && entry.Severity == Error {
			entry.Severity = Warning
			newEntries = append(newEntries, entry)
		} else {
			newEntries = append(newEntries, entry)
		}
	}
	log.Entries = newEntries
}
