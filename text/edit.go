// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines Extents and EditSets, which are used to describe
// additions, deletions, and modifications to be made to a string or text file.

package text

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
)

// -=-= Extent =-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// An Extent consists of two integers: a 0-based byte offset and a
// nonnegative length.  An Extent is used to specify a region of a string
// or file.  For example, given the string "ABCDEFG", the substring CDE could
// be specified by Extent{offset: 2, length: 3}.
type Extent struct {
	// Byte offset of the first character (0-based)
	Offset int `json:"offset"`
	// Length in bytes (nonnegative)
	Length int `json:"length"`
}

// OffsetPastEnd returns the offset of the first byte immediately beyond the
// end of this region.  For example, a region at offset 2 with length 3
// occupies bytes 2 through 4, so this method would return 5.
func (o *Extent) OffsetPastEnd() int {
	return o.Offset + o.Length
}

// Intersect returns the intersection (i.e., the overlapping region) of two
// intervals, or nil iff the intervals do not overlap.  A length-zero overlap
// is returned only if the two intervals are not adjacent.
func (o *Extent) Intersect(other *Extent) *Extent {
	start := max(o.Offset, other.Offset)
	end := min(o.OffsetPastEnd(), other.OffsetPastEnd())
	len := end - start
	if len < 0 {
		return nil
	}
	if len == 0 && o.IsAdjacentTo(other) {
		return nil
	}
	return &Extent{start, len}
}

// IsAdjacentTo returns true iff two intervals describe regions immediately
// next to one another, such as (offset 2, length 3) and (offset 5, length 1).
// Specifically, [a,b) is adjacent to [c,d) iff b == c or d == a.  Note that a
// length-zero interval is adjacent to itself.
func (o *Extent) IsAdjacentTo(other *Extent) bool {
	return o.OffsetPastEnd() == other.Offset ||
		other.OffsetPastEnd() == o.Offset
}

func (o *Extent) String() string {
	return fmt.Sprintf("offset %d, length %d", o.Offset, o.Length)
}

// -=-= EditSet -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

// An EditSet is a collection of changes to be made to a text file.  Each edit
// is comprised of an offset, a length, and a replacement string.
//
// Each edit replaces 0 or more characters at a given offset with a given
// string.  Characters can be inserted by using a position with length 0;
// characters can be deleted by using an empty replacement string.
//
// Edits are added to an EditSet via the Add method, and the edits in an
// EditSet can be applied to an input by invoking the ApplyTo method or
// one of the utility functions ApplyToString, ApplyToFile, or ApplyToReader.
type EditSet struct {
	edits []edit // edits are sorted by offset and are non-overlapping
}

type edit struct {
	Extent
	replacement string
}

// NewEditSet returns a new, empty EditSet.
func NewEditSet() *EditSet {
	return &EditSet{edits: []edit{}}
}

// RelativeToOffset returns a new edit whose offset is the offset of this edit
// minus the given offset, i.e., it is an edit relative to the given offset.
func (e *edit) RelativeToOffset(offset int) edit {
	return edit{
		Extent{
			Offset: e.Offset - offset,
			Length: e.Length,
		},
		e.replacement}
}

// overlaps returns true iff this edit overlaps the given interval
func (e *edit) overlaps(pos *Extent) bool {
	return e.Extent.Intersect(pos) != nil
}

// Add inserts an edit into this EditSet, returning an error if the edit has a
// negative offset or overlaps an edit previously added to this EditSet.
// FIXME(jeff): pos should be *Extent, not Extent
func (e *EditSet) Add(pos Extent, replacement string) error {
	if pos.Offset < 0 {
		return fmt.Errorf("edit has negative offset (%d)",
			pos.Offset)
	}

	// Insert edit into e.edits, keeping e.edits sorted by offset
	idx := len(e.edits)
	for i := len(e.edits) - 1; i >= 0; i-- {
		if e.edits[i].Offset >= pos.Offset {
			idx = i
		} else {
			break
		}
	}
	if idx > 0 && e.edits[idx-1].overlaps(&pos) {
		return fmt.Errorf("overlapping edit at offset %d", pos.Offset)
	}
	if idx < len(e.edits) && e.edits[idx].overlaps(&pos) {
		return fmt.Errorf("overlapping edit at offset %d", pos.Offset)
	}
	newEdit := edit{pos, replacement}
	e.edits = append(e.edits, newEdit)
	copy(e.edits[idx+1:], e.edits[idx:])
	e.edits[idx] = newEdit
	return nil
}

// SizeChange returns the total number of bytes that will be added or removed
// when this EditSet is applied.  A positive value indicates that bytes will be
// added; negative, bytes will be removed.  A zero value indicates that the
// total number of bytes will stay the same after the EditSet is applied.
func (e *EditSet) SizeChange() int64 {
	var total int64
	for _, edit := range e.edits {
		total += int64(len(edit.replacement) - edit.Length)
	}
	return total
}

func (e *edit) String() string {
	return "Replace " + e.Extent.String() +
		" with \"" + e.replacement + "\""
}

// String returns a human-readable description of this EditSet (for debugging).
func (e *EditSet) String() string {
	var buffer bytes.Buffer
	for _, edit := range e.edits {
		buffer.WriteString(edit.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

// ApplyTo reads from the given reader, applying the edits in this EditSet as
// it reads, and writes the output to the given writer.  It returns an error if
// there are edits with offsets beyond the end of the input or some other error
// occurs, such as an I/O error.
func (e *EditSet) ApplyTo(in io.Reader, out io.Writer) error {
	bufin := bufio.NewReader(in)
	bufout := bufio.NewWriter(out)
	return e.applyTo(bufin, bufout)
}

func (e *EditSet) applyTo(in *bufio.Reader, out *bufio.Writer) error {
	// This uses the same idea as the linear-time merge in Merge Sort to
	// apply the edits in this EditSet to the bytes from the input reader.
	defer out.Flush()
	offset := 0
	for _, edit := range e.edits {
		// Copy bytes preceding this edit
		bytesToWrite := int64(edit.Offset - offset)
		bytesWritten, err := io.CopyN(out, in, bytesToWrite)
		offset += int(bytesWritten)
		if bytesWritten < bytesToWrite {
			return fmt.Errorf("edit offset %d is beyond "+
				"the end of the file (%d bytes)",
				edit.Offset, offset)
		} else if err != nil {
			return err
		}
		// Write replacement
		out.WriteString(edit.replacement)
		// Skip bytes replaced by this edit
		bytesToWrite = int64((edit.Offset + edit.Length) - offset)
		bytesWritten, err = io.CopyN(ioutil.Discard, in, bytesToWrite)
		offset += int(bytesWritten)
		if bytesWritten < bytesToWrite {
			return fmt.Errorf("edit offset %d is beyond "+
				"the end of the file (%d bytes)",
				edit.Offset, offset)
		} else if err != nil {
			return err
		}
	}
	// Copy remaining bytes until end of file
	_, err := io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}

// CreatePatch creates a Patch from this EditSet.  A Patch can be output as a
// unified diff by invoking the Patch's Write method.
func (e *EditSet) CreatePatch(in io.Reader) (result *Patch, err error) {
	return createPatch(e, in)
}

// CreatePatchForFile reads bytes from a file, applying the edits in an EditSet
// and returning a Patch.
func CreatePatchForFile(es *EditSet, filename string) (*Patch, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return es.CreatePatch(file)
}

// ApplyToFile reads bytes from a file, applying the edits in an EditSet and
// returning the result as a slice of bytes.
func ApplyToFile(es *EditSet, filename string) ([]byte, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return ApplyToReader(es, file)
}

// ApplyToString reads bytes from a string, applying the edits in an EditSet
// and returning the result as a string.
func ApplyToString(es *EditSet, s string) (string, error) {
	bs, err := ApplyToReader(es, strings.NewReader(s))
	return string(bs), err
}

// ApplyToReader reads bytes from an io.Reader, applying the edits in an
// EditSet and returning the result as a slice of bytes.
func ApplyToReader(es *EditSet, in io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	err := es.ApplyTo(in, &buf)
	return buf.Bytes(), err
}
