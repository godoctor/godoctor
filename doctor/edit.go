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
