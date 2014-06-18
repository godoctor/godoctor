// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

// Use zz_* for test file and directory names, since those paths are in
// .gitignore.

const testFile = "zz_test.txt"
const testFile2 = "zz_test2.txt"
const testDir = "zz_test"

func TestCreateFile1(t *testing.T) {
	contents := "This is a file\nwith two lines"
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testFile, contents); err != nil {
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadFile(testFile)
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	if string(bytes) != contents {
		os.Remove(testFile)
		t.Fatal("Incorrect file contents:\n", string(bytes))
	}
	os.Remove(testFile)
}

func TestCreateFile2Remove(t *testing.T) {
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testFile, ""); err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateFile(testFile, "x"); err == nil {
		os.Remove(testFile)
		t.Fatal("Create over existing file should have failed")
	}
	if err := fs.Remove(testFile); err != nil {
		t.Fatal(err)
	}
	if err := fs.CreateFile(testFile, ""); err != nil {
		t.Fatal(err)
	}
	os.Remove(testFile)
}

func TestCreateFile3(t *testing.T) {
	if err := os.Mkdir(testDir, os.ModeDir|0775); err != nil {
		t.Fatal(err)
	}
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(testDir, "x"); err == nil {
		os.Remove(testDir)
		t.Fatal("Create over directory should have failed")
	}
	os.Remove(testDir)
}

func TestRenameFile(t *testing.T) {
	if err := os.Mkdir(testDir, os.ModeDir|0775); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("%s/%s", testDir, testFile)
	fs := NewLocalFileSystem()
	if err := fs.CreateFile(path, ""); err != nil {
		t.Fatal(err)
	}
	fs.Rename(path, testFile2)
	newPath := fmt.Sprintf("%s/%s", testDir, testFile2)
	if err := fs.CreateFile(newPath, "x"); err == nil {
		os.RemoveAll(testDir)
		t.Fatal("Create over renamed file should have failed")
	}
	if err := os.RemoveAll(testDir); err != nil {
		t.Fatal(err)
	}
}

func TestEditedFileSystem(t *testing.T) {
	contents := "123456789\nABCDEFGHIJ"
	lfs := NewLocalFileSystem()
	if err := lfs.CreateFile(testFile, contents); err != nil {
		t.Fatal(err)
	}
	es := NewEditSet()
	es.Add(OffsetLength{3, 5}, "xyz")
	expected := "123xyz9\nABCDEFGHIJ"
	fs := NewEditedFileSystem(map[string]*EditSet{testFile: es})
	editedFile, err := fs.OpenFile(testFile)
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	bytes, err := ioutil.ReadAll(editedFile)
	editedFile.Close()
	if err != nil {
		os.Remove(testFile)
		t.Fatal(err)
	}
	if string(bytes) != expected {
		os.Remove(testFile)
		t.Fatal("Incorrect file contents:\n", string(bytes))
	}
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}
}

func TestVirtualFileSystem(t *testing.T) {
	files := map[string]string{"a": "AAA", "b": "BBB"}
	fs := NewVirtualFileSystem()
	for file, contents := range files {
		if err := fs.CreateFile(file, contents); err != nil {
			t.Fatal(err)
		}
	}
	for file, expected := range files {
		r, err := fs.OpenFile(file)
		if err != nil {
			t.Fatal(err)
		}
		bytes, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(bytes) != expected {
			t.Fatal("File contents mismatch: ", string(bytes))
		}
	}
}
