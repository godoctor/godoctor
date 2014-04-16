// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

/* -=-=- File System Interface -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

type FileSystem interface {
	// ReadDir returns a slice of os.FileInfo, sorted by Name,
	// describing the content of the named directory.
	ReadDir(dir string) (fi []os.FileInfo, err error)

	// OpenFile opens a file (not a directory) for reading.
	OpenFile(path string) (r io.ReadCloser, err error)

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

type LocalFileSystem struct{}

func (fs *LocalFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

func (fs *LocalFileSystem) OpenFile(path string) (r io.ReadCloser, err error) {
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

func (fs *LocalFileSystem) CreateDirectory(path string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Path already exists: %s", path)
	}
	return os.Mkdir(path, os.ModeDir)
}

func (fs *LocalFileSystem) Rename(path, newName string) error {
	return os.Rename(path, newName)
}

func (fs *LocalFileSystem) Remove(path string) error {
	return os.Remove(path)
}

/* -=-=- Virtual File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

type VirtualFileSystem struct {
	files map[string]string
}

func isInGoRoot(path string) bool {
	return strings.HasPrefix(path, os.Getenv("GOROOT"))
}

func (fs *VirtualFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	if isInGoRoot(path) {
		return ioutil.ReadDir(path)
	} else {
		return []os.FileInfo{}, nil
	}
}

func (fs *VirtualFileSystem) OpenFile(path string) (r io.ReadCloser, err error) {
	if isInGoRoot(path) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return f, nil
	} else {
		_, fname := filepath.Split(path)
		contents, ok := fs.files[fname]
		if !ok {
			return nil,
				fmt.Errorf("Virtual file not found: %s", fname)
		}
		return ioutil.NopCloser(strings.NewReader(contents)), nil
	}
}

func (fs *VirtualFileSystem) CreateFile(path, contents string) error {
	if _, ok := fs.files[path]; ok {
		return fmt.Errorf("File already exists: %s", path)
	}
	if fs.files == nil {
		fs.files = map[string]string{}
	}
	fs.files[path] = contents
	return nil
}

func (fs *VirtualFileSystem) Rename(path, newName string) error {
	if _, ok := fs.files[path]; !ok {
		return fmt.Errorf("File does not exist: %s", path)
	}
	if _, ok := fs.files[newName]; ok {
		return fmt.Errorf("File already exists: %s", newName)
	}
	fs.files[newName] = fs.files[path]
	delete(fs.files, path)
	return nil
}

func (fs *VirtualFileSystem) Remove(path string) error {
	if _, ok := fs.files[path]; !ok {
		return fmt.Errorf("File does not exist: %s", path)
	}
	delete(fs.files, path)
	return nil
}

/* -=-=- File System Changes -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

type FileSystemChange interface {
	ExecuteUsing(FileSystem) error
}

type fsCreateFile struct {
	path, contents string
}

func (chg *fsCreateFile) ExecuteUsing(fs FileSystem) error {
	return fs.CreateFile(chg.path, chg.contents)
}

type fsRemove struct {
	path string
}

func (chg *fsRemove) ExecuteUsing(fs FileSystem) error {
	return fs.Remove(chg.path)
}

type fsRename struct {
	path, newName string
}

func (chg *fsRename) ExecuteUsing(fs FileSystem) error {
	return fs.Rename(chg.path, chg.newName)
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
