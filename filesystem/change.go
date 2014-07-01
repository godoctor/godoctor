// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines changes that can be made to a FileSystem.

package filesystem

import (
	"fmt"

	"path/filepath"
)

// A FileSystemChange describes a change to be made to the file system:
// renaming, creating, or deleting a file/directory.
type FileSystemChange interface {
	ExecuteUsing(FileSystem) error
	String(relativeTo string) string
}

type CreateFile struct {
	Path, Contents string
}

func (chg *CreateFile) ExecuteUsing(fs FileSystem) error {
	return fs.CreateFile(chg.Path, chg.Contents)
}

func (chg *CreateFile) String(relativeTo string) string {
	return fmt.Sprintf("create %s", filepath.ToSlash(relative(chg.Path, relativeTo)))
}

type Remove struct {
	Path string
}

func (chg *Remove) ExecuteUsing(fs FileSystem) error {
	return fs.Remove(chg.Path)
}

func (chg *Remove) String(relativeTo string) string {
	return fmt.Sprintf("remove %s", filepath.ToSlash(relative(chg.Path, relativeTo)))
}

type Rename struct {
	Path, NewName string
}

func (chg *Rename) ExecuteUsing(fs FileSystem) error {
	return fs.Rename(chg.Path, chg.NewName)
}

func (chg *Rename) String(relativeTo string) string {
	return fmt.Sprintf("rename %s %s", filepath.ToSlash(relative(chg.Path, relativeTo)), chg.NewName)
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
	dir, _ := filepath.Split(filePath)
	return dir == ""
}

func relative(path, relativeTo string) string {
	relativeTo, err := filepath.Abs(relativeTo)
	if err != nil {
		return path
	}
	result, err := filepath.Rel(relativeTo, path)
	if err != nil {
		return path
	}
	return result
}
