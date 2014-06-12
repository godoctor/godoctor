// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

/* -=-=- File System Interface -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

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

type EditedFileSystem struct {
	LocalFileSystem
	Edits map[string]EditSet
}

func NewEditedFileSystem(edits map[string]EditSet) *EditedFileSystem {
	return &EditedFileSystem{Edits: edits}
}

func NewSingleEditedFileSystem(filename, contents string) (*EditedFileSystem, error) {
	size, err := sizeOf(filename)
	if err != nil {
		return nil, err
	}
	es := NewEditSet()
	es.Add(OffsetLength{0, size}, contents)
	return NewEditedFileSystem(map[string]EditSet{filename: es}), nil
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
	panic("Remove unsupported")
}

/* -=-=- Virtual File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

type VirtualFileSystem struct {
	files map[string]string
}

func NewVirtualFileSystem() *VirtualFileSystem {
	return &VirtualFileSystem{files: map[string]string{}}
}

func isInGoRoot(path string) bool {
	goRoot := os.Getenv("GOROOT")
	if goRoot == "" {
		return false
	}
	return strings.HasPrefix(path, goRoot)
}

func (fs *VirtualFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	if isInGoRoot(path) {
		return ioutil.ReadDir(path)
	} else {
		return fs.fileInfos(), nil
	}
}

func (fs *VirtualFileSystem) fileInfos() []os.FileInfo {
	now := time.Time{} // Zero time
	result := make([]os.FileInfo, len(fs.files))
	for name, contents := range fs.files {
		fi := &vfsFileInfo{
			name:    name,
			size:    int64(len(contents)),
			modTime: now,
		}
		result = append(result, fi)
	}
	return result
}

type vfsFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (fi *vfsFileInfo) Name() string       { return fi.name }
func (fi *vfsFileInfo) Size() int64        { return fi.size }
func (fi *vfsFileInfo) Mode() os.FileMode  { return 0777 }
func (fi *vfsFileInfo) ModTime() time.Time { return fi.modTime }
func (fi *vfsFileInfo) IsDir() bool        { return false }
func (fi *vfsFileInfo) Sys() interface{}   { return nil }

func (fs *VirtualFileSystem) OpenFile(path string) (io.ReadCloser, error) {
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
	String() string
}

type fsCreateFile struct {
	path, contents string
}

func (chg *fsCreateFile) ExecuteUsing(fs FileSystem) error {
	return fs.CreateFile(chg.path, chg.contents)
}

func (chg *fsCreateFile) String() string {
	return fmt.Sprintf("create %s", chg.path)
}

type fsRemove struct {
	path string
}

func (chg *fsRemove) ExecuteUsing(fs FileSystem) error {
	return fs.Remove(chg.path)
}

func (chg *fsRemove) String() string {
	return fmt.Sprintf("remove %s", chg.path)
}

type fsRename struct {
	path, newName string
}

func (chg *fsRename) ExecuteUsing(fs FileSystem) error {
	return fs.Rename(chg.path, chg.newName)
}

func (chg *fsRename) String() string {
	return fmt.Sprintf("rename %s %s", chg.path, chg.newName)
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
