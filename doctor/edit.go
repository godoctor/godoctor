// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines EditSets, which describe changes (additions, deletions,
// and modifications) to be made to a text file.

package doctor

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"syscall"
)

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
	OffsetLength
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
		OffsetLength{
			Offset: e.Offset - offset,
			Length: e.Length,
		},
		e.replacement}
}

// Add inserts an edit into this EditSet, returning an error if the edit has a
// negative offset or overlaps an edit previously added to this EditSet.
func (e *EditSet) Add(position OffsetLength, replacement string) error {
	// Check for negative-offset or overlapping edits
	if position.Offset < 0 {
		return fmt.Errorf("edit has negative offset (%d)",
			position.Offset)
	}

	// Insert edit into e.edits, keeping e.edits sorted by offset
	var pos int = len(e.edits)
	for i := len(e.edits) - 1; i >= 0; i-- {
		if e.edits[i].Offset >= position.Offset {
			pos = i
		} else {
			break
		}
	}
	if pos > 0 && e.edits[pos-1].OffsetPastEnd() > position.Offset {
		return fmt.Errorf("overlapping edit at offset %d",
			position.Offset)
	}
	newEdit := edit{position, replacement}
	e.edits = append(e.edits, newEdit)
	copy(e.edits[pos+1:], e.edits[pos:])
	e.edits[pos] = newEdit
	return nil
}

func (e *edit) String() string {
	return "Replace " + e.OffsetLength.String() +
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
	var offset int = 0
	for _, edit := range e.edits {
		// Copy bytes preceding this edit
		var bytesToWrite int64 = int64(edit.Offset - offset)
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
		log.Fatal(err)
	}

	defer file.Close()

	return es.CreatePatch(file)
}

// ApplyToFile reads bytes from a file, applying the edits in an EditSet and
// returning the result as a slice of bytes.
func ApplyToFile(es *EditSet, filename string) ([]byte, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return ApplyToReader(es, file)
}

// ApplyToFile reads bytes from a string, applying the edits in an EditSet and
// returning the result as a string.
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
