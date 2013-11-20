// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains an implementation of the greedy longest common
// subsequence/shortest edit script (LCS/SES) algorithm described in
// Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations"
//
// It also contains support for creating unified diffs (i.e., patch files).
// The unified diff format is documented in the POSIX standard (IEEE 1003.1),
// "diff - compare two files", section: "Diff -u or -U Output Format"
// http://pubs.opengroup.org/onlinepubs/9699919799/utilities/diff.html

// Contributors: Jeff Overbey

package doctor

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

/* -=-=- Myers Diff Algorithm Implementation -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// Diff creates an EditSet containing the minimum number of line additions and
// deletions necessary to change a into b.  Typically, both a and b will be
// slices containing \n-terminated lines of a larger string, although it is
// also possible compute character-by-character diffs by splitting a string on
// UTF-8 boundaries.  The resulting EditSet is constructed so that it can be
// applied to the string produced by strings.Join(a, "").
//
// Every edit in the resulting EditSet starts at an offset corresponding to the
// first character on a line.  Every edit in the EditSet is either (1) a
// deletion, i.e., its length is the length of the current line and its
// replacement text is the empty string, or (2) an addition, i.e., its length
// is 0 and its replacement text is a single line to insert.
//
// The implementation follows the pseudocode in Myers' paper (cited above)
// fairly closely.
func Diff(a []string, b []string) EditSet {
	return diff(a, b)
}

// Internal implementation of Diff.  Returns an editSet (which includes non-API
// methods like newEditIter) rather than an EditSet (which is API).
func diff(a []string, b []string) *editSet {
	n := len(a)
	m := len(b)
	max := m + n
	if n == 0 && m == 0 {
		return &editSet{}
	} else if n == 0 {
		result := &editSet{}
		replacement := strings.Join(b, "")
		if replacement != "" {
			result.Add(OffsetLength{0, 0}, replacement)
		}
		return result
	} else if m == 0 {
		result := &editSet{}
		length := len(strings.Join(a, ""))
		if length > 0 {
			result.Add(OffsetLength{0, length}, "")
		}
		return result
	}
	vs := make([][]int, 0, max)
	v := make([]int, 2*max, 2*max)
	offset := max
	v[offset+1] = 0
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x, y int
			var vert bool
			if k == -d || k != d &&
				abs(v[offset+k-1]) < abs(v[offset+k+1]) {
				x = abs(v[offset+k+1])
				vert = false
			} else {
				x = abs(v[offset+k-1]) + 1
				vert = true
			}
			y = x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			if vert {
				v[offset+k] = -x
			} else {
				v[offset+k] = x
			}
			if x >= n && y >= m {
				// length of SES is D
				vs = append(vs, v)
				return constructEditSet(a, b, vs)
			}
		}
		v_copy := make([]int, len(v))
		copy(v_copy, v)
		vs = append(vs, v_copy)
	}
	panic("Length of SES longer than max (internal error)")
}

// Abs returns the absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	} else {
		return n
	}
}

// ConstructEditSet is a utility method invoked by Diff upon completion.  It
// uses the matrix vs (computed by Diff) to compute a sequence of deletions and
// additions.
func constructEditSet(a []string, b []string, vs [][]int) *editSet {
	n := len(a)
	m := len(b)
	max := m + n
	offset := max
	result := &editSet{}
	k := n - m
	for len(vs) > 1 {
		v := vs[len(vs)-1]
		v_k := v[offset+k]
		x := abs(v_k)
		y := x - k

		vs = vs[:len(vs)-1]
		v = vs[len(vs)-1]
		if v_k > 0 {
			k++
		} else {
			k--
		}
		next_v_k := v[offset+k]
		next_x := abs(next_v_k)
		next_y := next_x - k

		if v_k > 0 {
			// Insert
			charsToCopy := y - next_y - 1
			insertOffset := x - charsToCopy
			ol := OffsetLength{offsetOfString(insertOffset, a), 0}
			copyOffset := y - charsToCopy - 1
			replaceWith := b[copyOffset : copyOffset+1]
			replacement := strings.Join(replaceWith, "")
			if len(replacement) > 0 {
				result.Add(ol, replacement)
			}
		} else {
			// Delete
			charsToCopy := x - next_x - 1
			deleteOffset := x - charsToCopy - 1
			ol := OffsetLength{
				offsetOfString(deleteOffset, a),
				len(a[deleteOffset])}
			replaceWith := ""
			if ol.Length > 0 {
				result.Add(ol, replaceWith)
			}
		}
	}
	return result
}

// OffsetOfString returns the byte offset of the substring ss[index] in the
// string strings.Join(ss, "")
func offsetOfString(index int, ss []string) int {
	result := 0
	for i := 0; i < index; i++ {
		result += len(ss[i])
	}
	return result
}

/* -=-=- Unified Diff Support =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// Number of leading/trailing context lines in a unified diff
const num_ctx_lines int = 3

// A Patch is an object representing a unified diff.  It can be created from an
// EditSet by invoking the CreatePatch method.
//
// Patch implements the EditSet interface, so a patch can be applied just as
// any other EditSet can.  However, patches are read-only; the Add method will
// always return an error.
type Patch struct {
	filename string
	hunks    []*hunk
}

func (p *Patch) Add(OffsetLength, string) error {
	return errors.New("Add cannot be called on Patch (read-only)")
}

func (p *Patch) ApplyTo(in io.Reader, out io.Writer) error {
	panic("Not implemented")
}

func (p *Patch) CreatePatch(filename string, in io.Reader) (*Patch, error) {
	return p, nil
}

func (p *Patch) String() string {
	var result bytes.Buffer
	p.Write("filename", "filename", &result)
	return result.String()
}

// Add appends a hunk to this patch.  It is the caller's responsibility to
// ensure that hunks are added in the correct order.
func (p *Patch) add(hunk *hunk) {
	p.hunks = append(p.hunks, hunk)
}

// Write writes a unified diff to the given io.Writer.  The given filenames
// are used in the diff output.
func (p *Patch) Write(origFile string, newFile string, out io.Writer) error {
	writer := bufio.NewWriter(out)
	defer writer.Flush()
	if len(p.hunks) > 0 {
		fmt.Fprintf(writer, "--- %s\n+++ %s\n", origFile, newFile)
		lineOffset := 0
		for _, hunk := range p.hunks {
			adjust, err := writeDiffHunk(hunk, lineOffset, writer)
			if err != nil {
				return err
			}
			lineOffset += adjust
		}
	}
	return nil
}

// WriteUnifiedDiffHunk writes a single hunk in unified diff format.  If the
// edits in that hunk add lines, it returns the number of lines added; if the
// edits delete lines, it returns a negative number indicating the number of
// lines deleted (0 - number of lines deleted).  If the edits in the hunk do
// not change the number of lines, returns 0.
func writeDiffHunk(h *hunk, outputLineOffset int, out io.Writer) (int, error) {
	// Determine the lines in this hunk before and after applying edits
	origLines, newLines, err := computeLines(h)
	if err != nil {
		return 0, err
	}

	// Write the unified diff header
	numOrigLines := lenWithoutLastIfEmpty(origLines)
	numNewLines := lenWithoutLastIfEmpty(newLines)
	if _, err = fmt.Fprintf(out, "@@ -%d,%d +%d,%d @@\n",
		h.startLine, numOrigLines,
		h.startLine+outputLineOffset, numNewLines); err != nil {
		return 0, err
	}

	// Create an iterator that will traverse deletions and additions
	it := diff(origLines, newLines).newEditIter()

	// For each line in the original file, add one or more lines to the
	// unified diff output
	offset := 0
	for i, line := range origLines {
		if it.edit() == nil || it.edit().Offset > offset {
			// This line was not affected by any edits
			if i < len(origLines)-1 || line != "" {
				fmt.Fprintf(out, " %s", origLines[i])
			}
		} else {
			// This line was deleted (and possibly replaced by a
			// different line), or one or more lines were inserted
			// before this line
			deleted := false
			for it.edit() != nil && it.edit().Offset == offset ||
				it.edit() != nil && i == len(origLines)-1 {
				edit := it.edit()
				if edit.Length > 0 {
					// Delete line
					line := origLines[i]
					fmt.Fprintf(out, "-%s", line)
					if !strings.HasSuffix(line, "\n") {
						fmt.Fprintf(out, "\n"+
							"\\ No newline at "+
							"end of file\n")
					}
					deleted = true
				} else if edit.replacement != "" {
					// Insert line
					repl := edit.replacement
					fmt.Fprintf(out, "+%s", repl)
					if !strings.HasSuffix(repl, "\n") {
						fmt.Fprintf(out, "\n"+
							"\\ No newline at "+
							"end of file\n")
					}

				}
				it.moveToNextEdit()
			}
			if !deleted {
				if i < len(origLines)-1 || line != "" {
					fmt.Fprintf(out, " %s", origLines[i])
				}
			}
		}
		offset += len(line)
	}
	return numNewLines - numOrigLines, nil
}

// If the last string in the slice is the empty string, returns len(ss)-1;
// otherwise, returns len(ss).
func lenWithoutLastIfEmpty(ss []string) int {
	if len(ss) > 0 && ss[len(ss)-1] == "" {
		return len(ss) - 1
	} else {
		return len(ss)
	}
}

// ComputeLines computes the text that will result from applying the edits in
// this hunk, then returns both the original text and the new text split into
// lines on \n boundaries.  It returns a non-nil error if the edits in the hunk
// cannot be applied.
func computeLines(h *hunk) (origLines []string, newLines []string, err error) {
	hunk := h.hunk.String()
	newText, err := ApplyToString(&editSet{edits: h.edits}, hunk)
	if err != nil {
		return
	}

	origLines = strings.SplitAfter(hunk, "\n")
	newLines = strings.SplitAfter(newText, "\n")

	numOrig, numNew := len(origLines), len(newLines)
	trailingCtxLines := 0
	for i := 0; i < min(numOrig, numNew); i++ {
		if origLines[numOrig-i-1] == newLines[numNew-i-1] {
			trailingCtxLines++
		} else {
			break
		}
	}
	linesToRemove := max(trailingCtxLines-num_ctx_lines, 0)

	origLines = origLines[:numOrig-linesToRemove]
	newLines = newLines[:numNew-linesToRemove]
	return
}

// Min returns the minimum of two integers.
func min(m, n int) int {
	if m < n {
		return m
	} else {
		return n
	}
}

// Max returns the maximum of two integers.
func max(m, n int) int {
	if m > n {
		return m
	} else {
		return n
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
	startLine   int          // 1-based line number of this hunk
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

// Returns true iff the given edit adds characters at the beginning of this line
// without modifying or deleting any characters in the line.
func (l *lineRdr) editAddsToStart(e *edit) bool {
	if e == nil {
		return false
	} else {
		return e.Offset == l.lineOffset && e.Length == 0
	}
}

// Returns true iff the given edit adds characters to, modifies, or deletes
// characters from the line that was most recently read.
func (l *lineRdr) currentLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else if l.err == io.EOF {
		return e.OffsetPastEnd() >= l.lineOffset
	} else {
		return e.Offset < l.offsetPastEnd() &&
			e.OffsetPastEnd() >= l.lineOffset
	}
}

// Returns true iff the given edit adds characters to, modifies, or deletes
// characters from the line following the line that was most recently read.
func (l *lineRdr) nextLineIsAffectedBy(e *edit) bool {
	if e == nil {
		return false
	} else if l.err == io.EOF {
		return false
	} else {
		return e.OffsetPastEnd() > l.offsetPastEnd()
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
func (e *editSet) newEditIter() *editIter {
	return &editIter{e.edits, 0}
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

// The CreatePatch method on editSet delegates to this method, which creates
// a Patch from an editSet.
func createPatch(e *editSet, in io.Reader) (result *Patch, err error) {
	result = &Patch{}

	if len(e.edits) == 0 {
		return
	}

	reader := newLineRdr(in) // Reads lines from the original file
	it := e.newEditIter()    // Traverses edits (in order)
	var hunk *hunk = nil     // Current hunk being added to
	var trailingCtxLines int // Number of unchanged lines at end of hunk

	// Iterate through each line, adding lines to a hunk if they are
	// affected by an edit or at most 2*num_ctx_lines+1 following an edit;
	// add edits to the hunk whenever the last offset affected by that edit
	// is on the current line
	for err = reader.readLine(); err == nil || err == io.EOF; err = reader.readLine() {
		if hunk == nil {
			// No hunk has been started, so start one as soon as
			// we find a line that is changed
			if reader.currentLineIsAffectedBy(it.edit()) {
				hunk = startHunk(reader)
				last := addEditsOnCurLine(hunk, reader, it)
				if !reader.nextLineIsAffectedBy(last) {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
				}
			}
		} else {
			// A hunk has been started; add the current line, and
			// terminate the hunk after the maximum number of
			// trailing context lines have been added
			hunk.addLine(reader.line)
			if reader.currentLineIsAffectedBy(it.edit()) {
				last := addEditsOnCurLine(hunk, reader, it)
				if !reader.nextLineIsAffectedBy(last) {
					if reader.editAddsToStart(last) {
						trailingCtxLines = 1
					} else {
						trailingCtxLines = 0
					}
				}
			} else {
				trailingCtxLines++
				if trailingCtxLines > 2*num_ctx_lines {
					result.add(hunk)
					hunk = nil
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	if hunk != nil {
		result.add(hunk)
	}
	err = nil
	return
}

// Beginning with the current edit marked by the iterator it, adds that edit
// to the hunk as well as all subsequent edits whose last affected offset
// is on the current line.  Returns the last edit added to the hunk, or if
// no edits were added, the current edit marked by the iterator.
func addEditsOnCurLine(hunk *hunk, reader *lineRdr, it *editIter) *edit {
	var lastEdit *edit = it.edit()
	for reader.currentLineIsAffectedBy(it.edit()) {
		if reader.nextLineIsAffectedBy(it.edit()) {
			return lastEdit
		} else {
			lastEdit = it.edit()
			hunk.addEdit(it.edit())
			it.moveToNextEdit()
		}
	}
	return lastEdit
}
