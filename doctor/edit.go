// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines the EditSet interface and a default implementation,
// including the NewEditSet method.  All Go refactorings/transformations return
// an EditSet, which describes what file(s) will be affected by a refactoring
// and exactly what characters in those files will be affected.

// Contributors: Jeff Overbey, Reed Allman

package doctor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
)

// An EditSet is a collection of changes to be made to a text file.  Each
// edit is comprised of an offset, a length, and a replacement string.
//
// The EditSet is populated by invoking the Add method.  Each edit replaces 0
// or more characters at a given offset with a given string.  Characters can be
// inserted by using a position with length 0; characters can be deleted by
// using "" for the replacement string.
//
// Edits can be applied by invoking one of the ApplyTo methods.
//
// ApplyTo reads a stream of characters from the given io.Reader, applying the
// edits in the EditSet to the stream and writing the result to the given
// io.Writer.
//
// ApplyToFile applies the edits in the EditSet to the given file, writing the
// result to the given io.Writer.
//
// ApplyToString is intended for testing purposes only.  It applies the edits
// in the EditSet to a given string, returning the resulting string; or if an
// error occurs, a description of the error is returned, rather than the
// modified string contents.
//
// The String method returns a description of this EditSet (for debugging).
//
type EditSet interface {
	// FIXME(jeff) Can we delete this method?  edit objects should not be exposed
	Edits() map[string][]edit
	Add(file string, position OffsetLength, replacement string) error
	ApplyTo(key string, in io.Reader, out io.Writer) error
	ApplyToFile(filename string, out io.Writer) error
	ApplyToString(key string, s string) (string, error)
	CreatePatch(key string, in io.Reader) (*Patch, error)
	String() string
}

type edit struct {
	OffsetLength
	replacement string
}

type editSet struct {
	//where [key] is generally a file name, see Add
	edits map[string][]edit
}

// NewEditSet returns a new, empty EditSet.
func NewEditSet() EditSet {
	return &editSet{edits: make(map[string][]edit, 1)}
}

func (e *edit) RelativeToOffset(offset int) edit {
	return edit{
		OffsetLength{
			Offset: e.Offset - offset,
			Length: e.Length,
		},
		e.replacement}
}

//TODO (reed)
//Method to consider abstracting away some of the apply to
//stuff into the driver, mainly because there's
//no need to have 7 methods to do JSON, stdout & writing
//
func (e *editSet) Edits() map[string][]edit {
	return e.edits
}

//Adds an edit to the editset, mapping to the appropriate file
//
func (e *editSet) Add(file string, position OffsetLength, replacement string) error {
	//TODO meh, kind of don't like that it's not in place, but [index][index] is bad
	// Check for negative-offset or overlapping edits
	if position.Offset < 0 {
		return fmt.Errorf("edit has negative offset (%d)", position.Offset)
	}

	fedits := e.edits[file]

	var pos int = len(fedits)
	for i := len(fedits) - 1; i >= 0; i-- {
		if fedits[i].Offset >= position.Offset {
			pos = i
		} else {
			break
		}
	}
	if pos > 0 && fedits[pos-1].OffsetPastEnd() > position.Offset {
		return fmt.Errorf("overlapping edit at offset %d", position.Offset)
	}
	newEdit := edit{position, replacement}
	fedits = append(fedits, newEdit)
	copy(fedits[pos+1:], fedits[pos:])
	fedits[pos] = newEdit

	e.edits[file] = fedits
	return nil
}

func (e *edit) String() string {
	return "Replace " + e.OffsetLength.String() +
		" with \"" + e.replacement + "\""
}

func (e *editSet) String() string {
	var buffer bytes.Buffer
	for filename, edits := range e.edits {
		buffer.WriteString("Edits for ")
		buffer.WriteString(filename)
		buffer.WriteString(":\n")
		for _, edit := range edits {
			buffer.WriteString("    ")
			buffer.WriteString(edit.String())
			buffer.WriteString("\n")
		}
	}
	return buffer.String()
}

//Applies all edits associated with a given filename
//
func (e *editSet) ApplyToFile(filename string, out io.Writer) error {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return e.ApplyTo(filename, file, out)
}

//Applies all of the edits to a string, mainly for debugging
//TODO this doesn't really work, takes s as a key to e.edits,
//so have to add the string w/ a key
//
func (e *editSet) ApplyToString(key string, s string) (string, error) {
	var reader io.Reader = strings.NewReader(s)
	var writer bytes.Buffer
	err := e.ApplyTo(key, reader, &writer)
	return writer.String(), err
}

//Takes the key (filename) in map of edits, applies changes to given writer
//
func (e *editSet) ApplyTo(filename string, in io.Reader, out io.Writer) error {
	bufin := bufio.NewReader(in)
	bufout := bufio.NewWriter(out)
	return e.applyTo(filename, bufin, bufout)
}

//TODO definitely don't think this is as intended. For string reasons, mainly
//@key is generally a filename, but can be any key given to map e.edits.
func (e *editSet) applyTo(key string, in *bufio.Reader, out *bufio.Writer) error {
	defer out.Flush()
	var offset int = 0

	//all edits for a given key
	for _, edit := range e.edits[key] {
		// Copy bytes preceding this edit
		for ; offset < edit.Offset; offset++ {
			byte, err := in.ReadByte()
			if err == io.EOF {
				return fmt.Errorf("edit offset %d is beyond "+
					"the end of the file (%d bytes)",
					edit.Offset, offset)
			} else if err != nil {
				return err
			} else {
				out.WriteByte(byte)
			}
		}
		// Write replacement
		out.WriteString(edit.replacement)
		// Skip bytes replaced by this edit
		for ; offset < (edit.Offset + edit.Length); offset++ {
			_, err := in.ReadByte()
			if err != nil {
				return err
			}
		}
	}
	// Copy remaining bytes until end of file
	for {
		byte, err := in.ReadByte()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		} else {
			out.WriteByte(byte)
		}
	}
	return nil
}

// -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// Number of leading/trailing context lines in a unified diff
const num_ctx_lines int = 3

// A Patch is an object representing a unified diff.  It can be created from an
// EditSet by invoking the CreatePatch method.
//
// Patch implements the EditSet interface, so a patch can be applied just as
// any other EditSet can.  However, patches are read-only; the Add method will
// always return an error.
//
// The Unified Diff format is documented in the POSIX standard (IEEE 1003.1),
// "diff - compare two files", section: "Diff -u or -U Output Format"
// http://pubs.opengroup.org/onlinepubs/9699919799/utilities/diff.html
type Patch struct {
	filename string
	hunks    []*hunk
}

// FIXME(jeff) produce correct output when no edits are applied
// FIXME(jeff) produce multi-file patches
// with one file, this doesn't quite match the intent of the EditSet interface

func (p *Patch) Edits() map[string][]edit {
	edits := []edit{}
	for _, hunk := range p.hunks {
		edits = append(edits, hunk.edits...)
	}
	return map[string][]edit{p.filename: edits}
}

func (p *Patch) Add(string, OffsetLength, string) error {
	return errors.New("Add cannot be called on Patch (read-only)")
}
func (p *Patch) ApplyTo(key string, in io.Reader, out io.Writer) error {
	panic("Not implemented")
}

func (p *Patch) ApplyToFile(filename string, out io.Writer) error {
	panic("Not implemented")
}

func (p *Patch) ApplyToString(key string, s string) (string, error) {
	panic("Not implemented")
}

func (p *Patch) CreatePatch(key string, in io.Reader) (*Patch, error) {
	return p, nil
}

func (p *Patch) String() string {
	var result bytes.Buffer
	p.Write(&result)
	return result.String()
}

// Add appends a hunk to this patch.  It is the caller's responsibility to
// ensure that hunks are added in the correct order.
func (p *Patch) add(hunk *hunk) {
	p.hunks = append(p.hunks, hunk)
}

// Write writes a unified diff to the given io.Writer.
func (p *Patch) Write(out io.Writer) error {
	writer := bufio.NewWriter(out)
	defer writer.Flush()
	lineOffset := 0
	for _, hunk := range p.hunks {
		adjust, err := writeUnifiedDiffHunk(hunk, lineOffset, writer)
		if err != nil {
			return err
		}
		lineOffset += adjust
	}
	return nil
}

// WriteUnifiedDiffHunk writes a single hunk in unified diff format.
func writeUnifiedDiffHunk(h *hunk, outputLineOffset int, out io.Writer) (int, error) {
	es := editSet{edits: map[string][]edit{"": h.edits}}
	var newTextBuffer bytes.Buffer
	hunk := h.hunk.Bytes()
	err := es.ApplyTo("", bytes.NewReader(hunk), &newTextBuffer)
	if err != nil {
		return 0, err
	}
	newText := newTextBuffer.Bytes()

	leadingCtx, deletions, additions, trailingCtx := findContext(hunk, newText)

	var result bytes.Buffer
	var lines, linesInOrigText, linesInNewText int = 0, 0, 0

	if lines, err = writePrefixed(&result, " ", leadingCtx, -1); err != nil {
		return linesInNewText - linesInOrigText, err
	}
	linesInOrigText += lines
	linesInNewText += lines

	if lines, err = writePrefixed(&result, "-", deletions, -1); err != nil {
		return linesInNewText - linesInOrigText, err
	}
	linesInOrigText += lines

	if lines, err = writePrefixed(&result, "+", additions, -1); err != nil {
		return linesInNewText - linesInOrigText, err
	}
	linesInNewText += lines

	if lines, err = writePrefixed(&result, " ", trailingCtx, num_ctx_lines); err != nil {
		return linesInNewText - linesInOrigText, err
	}
	linesInOrigText += lines
	linesInNewText += lines

	_, err = fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n%s",
		h.startLine, linesInOrigText,
		h.startLine+outputLineOffset, linesInNewText,
		result.String())
	return linesInNewText - linesInOrigText, err
}

// WritePrefixed writes at most maxLines lines of a given string (as a []byte)
// to the given io.Writer, with the given prefix prepended to each line.  If
// maxLines < 0, the entire string is written.
func writePrefixed(out io.Writer, prefix string, str []byte, maxLines int) (int, error) {
	r := newLineRdr(bytes.NewReader(str))
	lines := 0
	err := r.readLine()
	for {
		if err == nil {
			fmt.Fprintf(out, "%s%s", prefix, r.line)
			lines++
			if lines == maxLines {
				return lines, nil
			}
			err = r.readLine()
		} else if err == io.EOF {
			if r.line != "" {
				fmt.Fprintf(out, "%s%s", prefix, r.line)
				if !strings.HasSuffix(r.line, "\n") {
					fmt.Fprintf(out, "\n")
				}
				lines++
			}
			return lines, nil
		} else {
			return lines, err
		}
	}
}

// Identifies leading and trailing context in the given strings.  It is assumed
// that there are at most num_ctx_lines lines of leading context and at most
// 2*num_ctx_lines+1 lines of trailing context, based on implementation details
// in the process used to create a Patch from an editSet.
func findContext(a, b []byte) (leadingCtx, deletions, additions, trailingCtx []byte) {
	// Find leading context
	endOfLeadingContext := 0
	leadingCtxLines := 0
	newEnd := matchLine(a, b, endOfLeadingContext)
	for newEnd > endOfLeadingContext && leadingCtxLines < num_ctx_lines {
		endOfLeadingContext = newEnd
		leadingCtxLines++
		newEnd = matchLine(a, b, endOfLeadingContext)
	}

	// Find trailing context
	startOfTrailingContext := 0
	trailingCtxLines := 0
	newStart := matchLineFromEnd(a[endOfLeadingContext:],
		b[endOfLeadingContext:],
		startOfTrailingContext)
	for newStart > startOfTrailingContext && trailingCtxLines < 2*num_ctx_lines+1 {
		startOfTrailingContext = newStart
		trailingCtxLines++
		newStart = matchLineFromEnd(a[endOfLeadingContext:],
			b[endOfLeadingContext:],
			startOfTrailingContext)
	}
	startOfTrailingContextA := len(a) - startOfTrailingContext
	startOfTrailingContextB := len(b) - startOfTrailingContext

	// Return the four possible components of a unified diff hunk
	leadingCtx = a[:endOfLeadingContext]
	deletions = a[endOfLeadingContext:startOfTrailingContextA]
	additions = b[endOfLeadingContext:startOfTrailingContextB]
	trailingCtx = a[startOfTrailingContextA:]
	return
}

// Starting from the given offset, determines if the next line (up to \n) of
// a is identical to the next line of b.  If so, returns the offset of the
// character immediately following the \n at the end of that line (or the
// length of the string, if the end was reached before the line terminated).
// If the two lines do not match, startingOffset is returned.
func matchLine(a []byte, b []byte, startingOffset int) int {
	i := startingOffset
	for {
		if i == len(a) || i == len(b) {
			return i
		} else if i == len(a) || i == len(b) || a[i] != b[i] {
			return startingOffset
		} else {
			if a[i] == '\n' {
				return i + 1
			}
		}
		i++
	}
}

// Starting startingOffset+1 characters backwards from the end of both a and b,
// or startingOffset+2 characters if the character at startingOffset+1 is \n,
// reads backward to determines whether the preceding line (up to the next \n)
// of a is identical to the preceding line of b.  If so, returns the offset of
// the character immediately following the \n, i.e., the first character of the
// preceding line (or the length of the string, if the end was reached before
// the line terminated).  If the two lines do not match, startingOffset is
// returned.
func matchLineFromEnd(a []byte, b []byte, startingOffset int) int {
	aIndex := len(a) - startingOffset - 1
	bIndex := len(b) - startingOffset - 1
	for {
		if aIndex < 0 || bIndex < 0 || a[aIndex] != b[bIndex] {
			return startingOffset
		} else {
			if a[aIndex] == '\n' && aIndex < len(a)-startingOffset-1 {
				return len(a) - aIndex - 1
			}
			if aIndex == 0 && bIndex == 0 {
				return len(a)
			}
			aIndex--
			bIndex--
		}
	}
}

// A hunk represents a single hunk in a unified diff.  A hunk consists of all
// of the edits that affect a particular region of a file.  Typically, a hunk
// is written with num_ctx_lines (3) lines of context preceding and following
// the hunk.  So, edits are grouped: if there are more than 6 lines between two
// edits, they should be in separate hunks.  Otherwise, the two edits should be
// in the same hunk.
type hunk struct {
	startOffset int          // Offset of this hunk in the original file
	startLine   int          // 1-based line number of this hunk in the orig file
	numLines    int          // Number of lines modified by this hunk
	hunk        bytes.Buffer // Affected bytes from the original file
	edits       []edit       // Edits to be applied to hunk
}

func (h *hunk) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Line: %d\nOffset: %d\n", h.startLine, h.startOffset)
	fmt.Fprintf(&buf, "Number of Lines: %d\n", h.numLines)
	fmt.Fprintf(&buf, "Original Text:\nvvvvv\n%s\n^^^^^\n", h.hunk.String())
	fmt.Fprintf(&buf, "Edits:\n")
	for _, edit := range h.edits {
		fmt.Fprintf(&buf, "%s\n", edit.String())
	}
	return buf.String()
}

// AddLine adds a single line of text to the hunk.
func (h *hunk) addLine(line string) {
	h.hunk.WriteString(line)
	h.numLines++
}

// AddEdit appends a single edit to the hunk.  It is the caller's
// responsibility to ensure that edits are added in sorted order.
func (h *hunk) addEdit(e *edit) {
	h.edits = append(h.edits, e.RelativeToOffset(h.startOffset))
}

// A lineRdr reads lines, one at a time, from an io.Reader, keeping track of
// the 0-based offset and 1-based line number of the line.  It also keeps
// track of the previous num_ctx_lines lines that were read.  (This is used to
// create leading context for a unified diff hunk.)
type lineRdr struct {
	reader          *bufio.Reader
	line            string
	lineOffset      int
	lineNum         int
	err             error
	leadingCtxLines []string
}

// NewLineRdr creates a new lineRdr that reads from the given io.Reader.
func newLineRdr(in io.Reader) *lineRdr {
	return &lineRdr{
		reader:          bufio.NewReader(in),
		line:            "",
		lineOffset:      0,
		lineNum:         0,
		leadingCtxLines: []string{},
	}
}

// ReadLine reads a single line from the wrapped io.Reader.  When the end of
// the input is reached, it returns io.EOF.
func (l *lineRdr) readLine() error {
	if l.lineNum > 0 {
		if len(l.leadingCtxLines) == num_ctx_lines {
			l.leadingCtxLines = l.leadingCtxLines[1:]
		}
		l.leadingCtxLines = append(l.leadingCtxLines, l.line)
	}
	l.lineOffset += len(l.line)
	l.lineNum++
	l.line, l.err = l.reader.ReadString('\n')
	return l.err
}

// Returns the 0-based offset of the first character on the line following the
// line that was read, or the length of the file if the end was reached.
func (l *lineRdr) offsetPastEnd() int {
	return l.lineOffset + len(l.line)
}

// Returns true iff the given edit adds characters to, modifies, or deletes
// characters from the line that was most recently read.
func (l *lineRdr) currentLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else {
		return e.Offset < l.offsetPastEnd() &&
			e.OffsetPastEnd() >= l.lineOffset
	}
}

// StartHunk creats a new hunk, adding the current line and up to
// num_ctx_lines of leading context.
func startHunk(lr *lineRdr) *hunk {
	h := hunk{}
	h.startOffset = lr.lineOffset
	h.startLine = lr.lineNum
	h.numLines = 1

	for _, line := range lr.leadingCtxLines {
		h.startOffset -= len(line)
		h.startLine--
		h.numLines++
		h.hunk.WriteString(line)
	}

	h.hunk.WriteString(lr.line)
	return &h
}

// An iterator for []edit slices.
type editIter struct {
	edits     []edit
	nextIndex int
}

// Creates a new editIter with the first edit in the given file marked.
func (e *editSet) newEditIterator(filename string) *editIter {
	return &editIter{e.edits[filename], 0}
}

// Edit returns the edit currently under the mark, or nil if no edits remain.
func (e *editIter) edit() *edit {
	if e.nextIndex >= len(e.edits) {
		return nil
	} else {
		return &e.edits[e.nextIndex]
	}
}

// MoveToNextEdit moves the mark to the next edit.
func (e *editIter) moveToNextEdit() {
	e.nextIndex++
}

func (e *editSet) CreatePatch(key string, in io.Reader) (result *Patch, err error) {
	result = &Patch{}

	if len(e.edits) == 0 {
		return
	}

	const (
		HUNK_NOT_STARTED int = iota
		ADDING_TO_HUNK
		EDIT_ADDED_TO_HUNK
	)

	reader := newLineRdr(in)
	editIter := e.newEditIterator(key)
	curState := HUNK_NOT_STARTED
	var hunk *hunk = nil
	var trailingCtxLines int

	for err = reader.readLine(); err == nil; err = reader.readLine() {
		switch curState {
		case HUNK_NOT_STARTED:
			if reader.currentLineIsAffectedBy(editIter.edit()) {
				hunk = startHunk(reader)
				curState = ADDING_TO_HUNK
			} else {
				curState = HUNK_NOT_STARTED
			}
		case ADDING_TO_HUNK:
			hunk.addLine(reader.line)
			if reader.currentLineIsAffectedBy(editIter.edit()) {
				curState = ADDING_TO_HUNK
			} else {
				hunk.addEdit(editIter.edit())
				trailingCtxLines = 1
				editIter.moveToNextEdit()
				curState = EDIT_ADDED_TO_HUNK
			}

		case EDIT_ADDED_TO_HUNK:
			hunk.addLine(reader.line)
			if reader.currentLineIsAffectedBy(editIter.edit()) {
				curState = ADDING_TO_HUNK
			} else {
				trailingCtxLines++
				if trailingCtxLines < 2*num_ctx_lines {
					curState = EDIT_ADDED_TO_HUNK
				} else {
					result.add(hunk)
					hunk = nil
					curState = HUNK_NOT_STARTED
				}
			}
		}
	}
	if curState == ADDING_TO_HUNK || curState == EDIT_ADDED_TO_HUNK {
		if reader.line != "" {
			hunk.addLine(reader.line)
		}
		if curState == ADDING_TO_HUNK {
			hunk.addEdit(editIter.edit())
		}
		result.add(hunk)
	}
	if err == io.EOF {
		err = nil
	}
	return
}
