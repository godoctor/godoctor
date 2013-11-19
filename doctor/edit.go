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
	Add(position OffsetLength, replacement string) error
	ApplyTo(in io.Reader, out io.Writer) error
	CreatePatch(filename string, in io.Reader) (*Patch, error)
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

// Add inserts an edit into this editSet.
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

// ApplyTo reads from the given reader, applying the edits in this editSet and
// writing the output to the given writer
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

// CreatePatch creates a Patch from this editSet.  The patch is labeled with
// the given filename.
func (e *editSet) CreatePatch(filename string, in io.Reader) (result *Patch, err error) {
	return createPatch(e, filename, in)
}

func ApplyToFile(es EditSet, filename string) ([]byte, error) {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return ApplyToReader(es, file)
}

func ApplyToString(es EditSet, s string) (string, error) {
	bs, err := ApplyToReader(es, strings.NewReader(s))
	return string(bs), err
}

func ApplyToReader(es EditSet, in io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	err := es.ApplyTo(in, &buf)
	return buf.Bytes(), err
}
