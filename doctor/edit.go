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
	"fmt"
	"io"
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
// characters can be deleted by using "" for the replacement string.
//
// Edits are added to an EditSet via the Add method, and the edits in an
// EditSet can be applied to an input by invoking the ApplyTo method or
// one of the utility functions ApplyToString, ApplyToFile, or ApplyToReader.
type EditSet interface {
	// Add inserts an edit into this editSet, returning an error if the
	// edit has a negative offset or overlaps an edit previously added to
	// this EditSet.  If the EditSet is read-only (e.g., Patch implements
	// EditSet but cannot be modified), an error will always be returned.
	Add(position OffsetLength, replacement string) error

	// ApplyTo reads from the given reader, applying the edits in this
	// editSet as it reads, and writes the output to the given writer.
	// It returns an error if there are edits with offsets beyond the end
	// of the input or some other error occurs, such as an I/O error.
	ApplyTo(in io.Reader, out io.Writer) error

	// CreatePatch creates a Patch from this EditSet.  A Patch is itself
	// an EditSet -- it can be applied using ApplyTo -- or it can be output
	// as a unified diff by invoking the Patch's Write method.
	CreatePatch(in io.Reader) (*Patch, error)

	// String returns a human-readable description of this EditSet.
	String() string
}

type edit struct {
	OffsetLength
	replacement string
}

type editSet struct {
	edits []edit
}

// NewEditSet returns a new, empty EditSet.
func NewEditSet() EditSet {
	return &editSet{edits: []edit{}}
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

func (e *editSet) Add(position OffsetLength, replacement string) error {
	// Check for negative-offset or overlapping edits
	if position.Offset < 0 {
		return fmt.Errorf("edit has negative offset (%d)",
			position.Offset)
	}

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

func (e *editSet) String() string {
	var buffer bytes.Buffer
	for _, edit := range e.edits {
		buffer.WriteString(edit.String())
		buffer.WriteString("\n")
	}
	return buffer.String()
}

func (e *editSet) ApplyTo(in io.Reader, out io.Writer) error {
	bufin := bufio.NewReader(in)
	bufout := bufio.NewWriter(out)
	return e.applyTo(bufin, bufout)
}

func (e *editSet) applyTo(in *bufio.Reader, out *bufio.Writer) error {
	defer out.Flush()
	var offset int = 0
	for _, edit := range e.edits {
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

func (e *editSet) CreatePatch(in io.Reader) (result *Patch, err error) {
	return createPatch(e, in)
}

// ApplyToFile reads bytes from a file, applying the edits in an EditSet and
// returning the result as a slice of bytes.
func ApplyToFile(es EditSet, filename string) ([]byte, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return ApplyToReader(es, file)
}

// ApplyToFile reads bytes from a string, applying the edits in an EditSet and
// returning the result as a string.
func ApplyToString(es EditSet, s string) (string, error) {
	bs, err := ApplyToReader(es, strings.NewReader(s))
	return string(bs), err
}

// ApplyToReader reads bytes from an io.Reader, applying the edits in an
// EditSet and returning the result as a slice of bytes.
func ApplyToReader(es EditSet, in io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	err := es.ApplyTo(in, &buf)
	return buf.Bytes(), err
}
