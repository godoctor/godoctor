package doctor

// This file defines the EditSet interface and a default implementation,
// including the NewEditSet method.

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
	Add(file string, position OffsetLength, replacement string)
	ApplyTo(key string, in io.Reader, out io.Writer) error
	ApplyToAllFiles(out io.Writer) error
	WriteToAllFiles() error
	ApplyToFile(filename string, out io.Writer) error
	ApplyToString(s string) (string, error)
	String() string
}

type edit struct {
	OffsetLength
	replacement string
}

type editSet struct {
	edits map[string][]edit
}

// NewEditSet returns a new, empty EditSet.
func NewEditSet() EditSet {
	return &editSet{edits: make(map[string][]edit, 1)}
}

//Adds an edit to the editset, mapping to the appropriate file
//
func (e *editSet) Add(file string, position OffsetLength, replacement string) {
	//TODO see if need to malloc fedits?
	//TODO meh, kind of don't like that it's not in place, but [index][index] is bad
	fedits := e.edits[file]

	var pos int = len(fedits)
	for i := len(fedits) - 1; i >= 0; i-- {
		if fedits[i].offset > position.offset {
			pos = i
		} else {
			break
		}
	}
	newEdit := edit{position, replacement}
	fedits = append(fedits, newEdit)
	copy(fedits[pos+1:], fedits[pos:])
	fedits[pos] = newEdit

	e.edits[file] = fedits
}

func (e *edit) String() string {
	return "Replace " + e.OffsetLength.String() +
		" with \"" + e.replacement + "\""
}

func (e *editSet) String() string {
	var buffer bytes.Buffer
	for _, edits := range e.edits {
		for _, edit := range edits {
			buffer.WriteString(edit.String())
			buffer.WriteString("\n")
		}
	}
	return buffer.String()
}

//Applies all of the edits in an edit set
//
func (e *editSet) ApplyToAllFiles(out io.Writer) (err error) {
	for file, _ := range e.edits {
		err = e.ApplyToFile(file, out)
	}
	return
}

func (e *editSet) WriteToAllFiles() (err error) {
	var buf bytes.Buffer
	for file, _ := range e.edits {
		if err = e.ApplyToFile(file, &buf); err != nil {
			return err
		}

		if err := ioutil.WriteFile(file, buf.Bytes(), 0); err != nil {
			return err
		}
	}

	return
}

func (e *editSet) writeToFile(filename string) error {
	file, err := os.OpenFile(filename, syscall.O_RDWR, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	return e.ApplyTo(filename, file, file)
}

//Applies all of the edits in an edit set to a file (not good for multi files)
//TODO change to take a set of changes and a file?
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
//TODO this doesn't really work, takes s as a key to e.edits
//
func (e *editSet) ApplyToString(s string) (string, error) {
	var reader io.Reader = strings.NewReader(s)
	var writer bytes.Buffer
	err := e.ApplyTo(s, reader, &writer)
	return writer.String(), err
}

//TODO (reed) still thinks this doesn't exactly work as intended?
//
func (e *editSet) ApplyTo(key string, in io.Reader, out io.Writer) error {
	bufin := bufio.NewReader(in)
	bufout := bufio.NewWriter(out)
	return e.applyTo(key, bufin, bufout)
}

func (e *editSet) applyTo(key string, in *bufio.Reader, out *bufio.Writer) error {
	defer out.Flush()
	var offset int = 0
	for _, edit := range e.edits[key] {
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
	//TODO (reed's fault) uh move to above somehow?
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
