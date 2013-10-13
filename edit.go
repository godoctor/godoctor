package doctor

// This file defines the EditSet interface and a default implementation,
// including the NewEditSet method.

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
type EditSet interface {
	Add(position OffsetLength, replacement string)
	ApplyTo(in io.Reader, out io.Writer) error
	ApplyToFile(filename string, out io.Writer) error
	ApplyToString(s string) string
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
	var result editSet = editSet{edits: make([]edit, 0, 1)}
	return &result
}

func (e *editSet) Add(position OffsetLength, replacement string) {
	var pos int = len(e.edits)
	for i := len(e.edits) - 1; i >= 0; i-- {
		if e.edits[i].offset > position.offset {
			pos = i
		} else {
			break
		}
	}
	newEdit := edit{position, replacement}
	e.edits = append(e.edits, newEdit)
	copy(e.edits[pos+1:], e.edits[pos:])
	e.edits[pos] = newEdit
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

func (e *editSet) ApplyToFile(filename string, out io.Writer) error {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	return e.ApplyTo(file, out)
}

func (e *editSet) ApplyToString(s string) string {
	var reader io.Reader = strings.NewReader(s)
	var writer bytes.Buffer
	err := e.ApplyTo(reader, &writer)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	return writer.String()
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
		// Check for negative-offset or overlapping edits
		if edit.offset < 0 {
			return fmt.Errorf("Edit has negative offset (%d)",
				edit.offset)
		} else if offset > edit.offset {
			return fmt.Errorf("Overlapping edit at offset %d",
				edit.offset)
		}
		// Copy bytes preceding this edit
		for ; offset < edit.offset; offset++ {
			byte, err := in.ReadByte()
			if err == io.EOF {
				return fmt.Errorf("Edit offset %d is beyond "+
					"the end of the file (%d bytes)",
					edit.offset, offset)
			} else if err != nil {
				return err
			} else {
				out.WriteByte(byte)
			}
		}
		// Write replacement
		out.WriteString(edit.replacement)
		// Skip bytes replaced by this edit
		for ; offset < (edit.offset + edit.length); offset++ {
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
