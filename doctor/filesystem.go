// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a FileSystem interface and two implementations.  A
// FileSystem is supplied to the go/loader to read files, and it is also used
// by the refactoring driver to commit refactorings' changes to disk.

package doctor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

/* -=-=- File System Interface -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// A FileSystem provides the ability to read directories and files, as well as
// to create, rename, and remove files (if the file system is not read-only).
type FileSystem interface {
	// ReadDir returns a slice of os.FileInfo, sorted by Name,
	// describing the content of the named directory.
	ReadDir(dir string) ([]os.FileInfo, error)

	// OpenFile opens a file (not a directory) for reading.
	OpenFile(path string) (io.ReadCloser, error)

	// CreateFile creates a text file with the given contents and default
	// permissions.
	CreateFile(path, contents string) error

	// Rename changes the name of a file or directory.  newName should be a
	// bare name, not including a directory prefix; the existing file will
	// be renamed within its existing parent directory.
	Rename(path, newName string) error

	// Remove deletes a file or an empty directory.
	Remove(path string) error
}

/* -=-=- Local File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// LocalFileSystem implements the FileSystem interface and provides access to
// the local file system by delegating to the os and io/ioutil packages.
type LocalFileSystem struct{}

func NewLocalFileSystem() *LocalFileSystem {
	return &LocalFileSystem{}
}

func (fs *LocalFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

func (fs *LocalFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *LocalFileSystem) CreateFile(path, contents string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Path already exists: %s", path)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(file, contents); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return nil
}

func (fs *LocalFileSystem) Rename(oldPath, newName string) error {
	if !isBareFilename(newName) {
		return fmt.Errorf("newName must be a bare filename: %s",
			newName)
	}
	newPath := path.Join(path.Dir(oldPath), newName)
	return os.Rename(oldPath, newPath)
}

func (fs *LocalFileSystem) Remove(path string) error {
	return os.Remove(path)
}

/* -=-=- Edited File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// EditedFileSystem implements the FileSystem interface and provides access to
// a hypothetical version of the local file system after a refactoring's
// changes have been applied.  This can be supplied to go/loader to analyze a
// program after refactoring, without actually changing the program on disk.
// File/directory creation, renaming, and deletion are not currently supported.
type EditedFileSystem struct {
	LocalFileSystem
	Edits map[string]*EditSet
}

func NewEditedFileSystem(edits map[string]*EditSet) *EditedFileSystem {
	return &EditedFileSystem{Edits: edits}
}

func NewSingleEditedFileSystem(filename, contents string) (*EditedFileSystem, error) {
	size, err := sizeOf(filename)
	if err != nil {
		return nil, err
	}
	es := NewEditSet()
	es.Add(OffsetLength{0, size}, contents)
	return NewEditedFileSystem(map[string]*EditSet{filename: es}), nil
}

func sizeOf(filename string) (int, error) {
	if filename == os.Stdin.Name() {
		return 0, nil
	}
	f, err := os.Open(filename)
	if err != nil {
		//if os.IsNotExist(err) {
		//	return 0, nil
		//}
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := int(fi.Size())
	if int64(size) != fi.Size() {
		return 0, fmt.Errorf("File too large: %d bytes\n", fi.Size())
	}
	return size, nil
}

func (fs *EditedFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	localReader, err := fs.LocalFileSystem.OpenFile(path)
	editSet, ok := fs.Edits[path]
	if err != nil || !ok {
		return localReader, err
	}
	contents, err := ApplyToReader(editSet, localReader)
	if err != nil {
		return nil, err
	}
	if err := localReader.Close(); err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(contents)), nil
}

func (fs *EditedFileSystem) CreateFile(path, contents string) error {
	panic("CreateFile unsupported")
}

func (fs *EditedFileSystem) CreateDirectory(path string) error {
	panic("CreateDirectory unsupported")
}

func (fs *EditedFileSystem) Rename(path, newName string) error {
	panic("Rename unsupported")
}

func (fs *EditedFileSystem) Remove(path string) error {
	panic("Remove unsupallPackages(r.program) ported")
}

/* -=-=- File System Changes -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// A FileSystemChange describes a change to be made to the file system:
// renaming, creating, or deleting a file/directory.
type FileSystemChange interface {
	ExecuteUsing(FileSystem) error
	String(relativeTo string) string
}

type FSCreateFile struct {
	Path, Contents string
}

func (chg *FSCreateFile) ExecuteUsing(fs FileSystem) error {
	return fs.CreateFile(chg.Path, chg.Contents)
}

func (chg *FSCreateFile) String(relativeTo string) string {
	return fmt.Sprintf("create %s", relative(chg.Path, relativeTo))
}

type FSRemove struct {
	Path string
}

func (chg *FSRemove) ExecuteUsing(fs FileSystem) error {
	return fs.Remove(chg.Path)
}

func (chg *FSRemove) String(relativeTo string) string {
	return fmt.Sprintf("remove %s", relative(chg.Path, relativeTo))
}

type FSRename struct {
	Path, NewName string
}

func (chg *FSRename) ExecuteUsing(fs FileSystem) error {
	return fs.Rename(chg.Path, chg.NewName)
}

func (chg *FSRename) String(relativeTo string) string {
	return fmt.Sprintf("rename %s %s", relative(chg.Path, relativeTo), chg.NewName)
}

func Execute(fs FileSystem, changes []FileSystemChange) error {
	// TODO: the changes should be executed atomically (all-or-nothing),
	// but currently it can fail in the middle of execution
	for _, chg := range changes {
		if err := chg.ExecuteUsing(fs); err != nil {
			return err
		}
	}
	return nil
}

func isBareFilename(filePath string) bool {
	dir, _ := path.Split(filePath)
	return dir == ""
}

func relative(path, relativeTo string) string {
	relativeTo, err := filepath.Abs(relativeTo)
	if err != nil {
		return path
	}
	//fmt.Println("path: ", path, "relativeTo:", relativeTo)
	result, err := filepath.Rel(relativeTo, path)
	if err != nil {
		return path
	}
	return result
}
