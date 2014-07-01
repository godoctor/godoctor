// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file provides tests for all refactorings.  The testdata directory is
// structured as such:
//
//     testdata/
//         refactoring-name/
//             001-test-name/
//             002-test-name/
//
// To filter which refactorings are tested, run
//     go test -filter=something
// Then, only tests in directories containing "something" are run.  E.g.::
//     go test -filter=rename              # Only run rename tests
//     go test -filter=shortassign/003     # Only run shortassign test #3
//
// Refactorings are run on the files in each test directory; special comments
// in the .go files indicate what refactoring(s) to run and whether the
// refactorings are expected to succeed or fail.  Specifically, test files are
// annotated with markers of the form
//     //<<<<<name,startline,startcol,endline,endcol,arg1,arg2,...,argn,pass
// The name indicates the refactoring to run.  The next four fields specify a
// text selection on which to invoke the refactoring.  The arguments
// arg1,arg2,...,argn are passed as arguments to the refactoring (see
// Config.Args).  The last field is either "pass" or "fail", indicating whether
// the refactoring is expected to complete successfully or raise an error.  If
// the refactoring is expected to succeed, the resulting file is compared
// against a .golden file with the same name in the same directory.
//
// Each test directory (001-test-name, 002-test-name, etc.) is treated as the
// root of a Go workspace when its tests are run; i.e., the GOPATH is set to
// the test directory.  This allows it to define its own packages.  In such
// cases, the test directory is usually structured as follows:
//
//     testdata/
//         refactoring-name/
//             001-test-name/
//                 src/
//                     main.go
//                     main.golden
//                     package-name/
//                         package-file.go
//                         package-file.golden
//
// To additional options are available and are currently used only to test the
// Debug refactoring.  When it does not make sense to compare against a .golden
// file, include an empty file named filename.golden.ignoreOutput instead.  If
// the output will contain absolute paths to files in the test folder, include
// a file named filename.go.stripPaths, and the refactoring's output will be
// stripped of all occurrences of the absolute path to the .go file.

package refactoring

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang-refactoring.org/go-doctor/filesystem"
	"golang-refactoring.org/go-doctor/text"
)

const MARKER = "<<<<<"
const PASS = "pass"
const FAIL = "fail"

const MAIN_DOT_GO = "main.go"

const FSCHANGES_TXT = "fschanges.txt"

var filterFlag = flag.String("filter", "",
	"Only tests from directories containing this substring will be run")

const directory = "testdata/"

func TestRefactorings(t *testing.T) {
	testDirs, err := ioutil.ReadDir(directory)
	failIfError(err, t)
	for _, testDirInfo := range testDirs {
		// FIXME(jeff): Move refactoring tests into testdata/refactoring/x and remove this hack
		if testDirInfo.IsDir() && testDirInfo.Name() != "diff" {
			runAllTestsInSubdirectories(testDirInfo, t)
		}
	}
}

func runAllTestsInSubdirectories(testDirInfo os.FileInfo, t *testing.T) {
	testDirPath := filepath.Join(directory, testDirInfo.Name())
	subDirs, err := ioutil.ReadDir(testDirPath)
	failIfError(err, t)
	for _, subDirInfo := range subDirs {
		if subDirInfo.IsDir() {
			subDirPath := filepath.Join(testDirPath, subDirInfo.Name())
			if strings.Contains(subDirPath, *filterFlag) {
				runAllTestsInDirectory(subDirPath, t)
			}
		}
	}
}

// RunAllTests is a utility method that runs a set of refactoring tests
// based on markers in all of the files in subdirectories of a given directory
func runAllTestsInDirectory(directory string, t *testing.T) {
	files, err := recursiveReadDir(directory)
	failIfError(err, t)

	absolutePath, err := filepath.Abs(directory)
	failIfError(err, t)
	err = os.Setenv("GOPATH", absolutePath)
	failIfError(err, t)

	runTestsInFiles(directory, files, t)
}

// Assumes no duplication or circularity due to symbolic links
func recursiveReadDir(path string) ([]string, error) {
	result := []string{}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil {
		return []string{}, err
	}

	for _, fi := range fileInfos {
		if fi.IsDir() {
			current := result
			rest, err := recursiveReadDir(filepath.Join(path, fi.Name()))
			if err != nil {
				return []string{}, err
			}

			newLen := len(current) + len(rest)
			result = make([]string, newLen, newLen)
			copy(result, current)
			copy(result[len(current):], rest)
		} else {
			result = append(result, filepath.Join(path, fi.Name()))
		}
	}
	return result, err
}

func failIfError(err error, t *testing.T) {
	if err != nil {
		t.Fatal(err)
	}
}

func runTestsInFiles(directory string, files []string, t *testing.T) {
	markers := make(map[string][]string)
	for _, path := range files {
		if strings.HasSuffix(path, ".go") {
			markers[path] = extractMarkers(path, t)
		}
	}

	if len(markers) == 0 {
		pwd, _ := os.Getwd()
		pwd = filepath.Base(pwd)
		t.Fatalf("No <<<<< markers found in any files in %s", pwd)
	}

	for path, markersInFile := range markers {
		for _, marker := range markersInFile {
			runRefactoring(directory, path, marker, t)
		}
	}
}

func runRefactoring(directory string, filename string, marker string, t *testing.T) {
	refac, selection, remainder, passFail := splitMarker(filename, marker, t)

	r := GetRefactoring(refac)
	if r == nil {
		t.Fatalf("There is no refactoring named %s (from marker %s)", refac, marker)
	}

	shouldPass := (passFail == PASS)
	name := r.Description().Name

	cwd, _ := os.Getwd()
	absPath, _ := filepath.Abs(filename)
	relativePath, _ := filepath.Rel(cwd, absPath)
	fmt.Println(name, relativePath, selection.ShortString())

	mainFile := filepath.Join(directory, MAIN_DOT_GO)
	if !exists(mainFile, t) {
		mainFile = filepath.Join(filepath.Join(directory, "src"), MAIN_DOT_GO)
		if !exists(mainFile, t) {
			mainFile = filename
		}
	}

	mainFile, err := filepath.Abs(mainFile)
	if err != nil {
		t.Fatal(err)
	}

	args := InterpretArgs(remainder, r.Description().Params)

	fileSystem := &filesystem.LocalFileSystem{}
	config := &Config{
		FileSystem: fileSystem,
		Scope:      []string{mainFile},
		Selection:  selection,
		Args:       args,
		GoPath:     "", // FIXME(jeff): GOPATH
	}
	result := r.Run(config)
	if shouldPass && result.Log.ContainsErrors() {
		t.Log(result.Log)
		t.Fatalf("Refactoring produced unexpected errors")
	} else if !shouldPass && !result.Log.ContainsErrors() {
		t.Fatalf("Refactoring should have produced errors but didn't")
	}

	for filename, edits := range result.Edits {
		output, err := text.ApplyToFile(edits, filename)
		if err != nil {
			t.Fatal(err)
		}
		if shouldPass {
			checkResult(filename, string(output), t)

		}
	}
	//fmt.Println("filesystem changes",result.FSChanges)
	//  checkRenamedDir(result.RenameDir,"fschanges.txt")

	fsChangesFile := filepath.Join(directory, FSCHANGES_TXT)
	if !exists(fsChangesFile, t) {
		if len(result.FSChanges) > 0 {
			t.Fatalf("Refactoring returned file system changes, "+
				"but %s does not exist", fsChangesFile)
		}
	} else {
		bytes, err := ioutil.ReadFile(fsChangesFile)
		if err != nil {
			t.Fatal(err)
		}
		fschanges := removeEmptyLines(strings.Split(string(bytes), "\n"))
		if len(fschanges) != len(result.FSChanges) {
			t.Fatalf("Expected %d file system changes but got %d",
				len(fschanges), len(result.FSChanges))
		} else {
			for i, chg := range result.FSChanges {
				if chg.String(directory) != strings.TrimSpace(fschanges[i]) {
					t.Fatalf("FSChanges[%d]\nExpected: %s\nActual: %s", i, fschanges[i], chg.String(directory))
				}
			}
		}
	}
}

func removeEmptyLines(lines []string) []string {
	result := []string{}
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func exists(filename string, t *testing.T) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	} else {
		if os.IsNotExist(err) {
			return false
		} else {
			t.Fatal(err)
			return false
		}
	}
}

func checkResult(filename string, actualOutput string, t *testing.T) {
	if _, err := os.Stat(filename + ".ignoreOutput"); err == nil {
		return
	}
	if _, err := os.Stat(filename + ".stripPaths"); err == nil {
		dir := filepath.Dir(filename) + string(filepath.Separator)
		actualOutput = strings.Replace(actualOutput, dir, "", -1)
	}
	bytes, err := ioutil.ReadFile(filename + "lden")
	if err != nil {
		t.Fatal(err)
	}
	expectedOutput := strings.Replace(string(bytes), "\r\n", "\n", -1)
	actualOutput = strings.Replace(actualOutput, "\r\n", "\n", -1)

	if actualOutput != expectedOutput {
		fmt.Printf(">>>>> Output does not match %slden\n", filename)
		fmt.Println("EXPECTED OUTPUT")
		fmt.Println("vvvvvvvvvvvvvvv")
		fmt.Println(expectedOutput)
		fmt.Println("^^^^^^^^^^^^^^^")
		fmt.Println("ACTUAL OUTPUT")
		fmt.Println("vvvvvvvvvvvvv")
		fmt.Println(actualOutput)
		fmt.Println("^^^^^^^^^^^^^")
		lenExpected, lenActual := len(expectedOutput), len(actualOutput)
		if lenExpected != lenActual {
			fmt.Printf("Length of expected output is %d; length of actual output is %d\n",
				lenExpected, lenActual)
			minLen := lenExpected
			if lenActual < minLen {
				minLen = lenActual
			}
			for i := 0; i < minLen; i++ {
				if expectedOutput[i] != actualOutput[i] {
					fmt.Printf("Strings differ at index %d\n", i)
					fmt.Printf("Substrings starting at that index are:\n")
					fmt.Printf("Expected: [%s]\n", describe(expectedOutput[i:]))
					fmt.Printf("Actual: [%s]\n", describe(actualOutput[i:]))
					break
				}
			}
		}
		t.Fatalf("Refactoring test failed - %s", filename)
	}
}

//TODO Define after getting the value of Gopath
/*func checkRenamedDir(result.RenameDir []string,filename string) {

 if result.RenameDir != nil {

    bytes, err := ioutil.ReadFile(filename)
       if err != nil {
		t.Fatal(err)
	}
   expectedoutput :=  string(bytes)

}

}
*/
func describe(s string) string {
	// FIXME: Jeff: Handle other non-printing characters
	if len(s) > 10 {
		s = s[:10]
	}
	s = strings.Replace(s, "\n", "\\n", -1)
	s = strings.Replace(s, "\r", "\\r", -1)
	s = strings.Replace(s, "\t", "\\t", -1)
	return s
}

func splitMarker(filename string, marker string, t *testing.T) (refac string, selection *text.Selection, remainder []string, result string) {
	filename, err := filepath.Abs(filename)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Split(marker, ",")
	if len(fields) < 6 {
		t.Fatalf("Marker is invalid (must contain >= 5 fields): %s", marker)
	}
	refac = fields[0]
	startLine := parseInt(fields[1], t)
	startCol := parseInt(fields[2], t)
	endLine := parseInt(fields[3], t)
	endCol := parseInt(fields[4], t)
	selection = &text.Selection{filename,
		startLine, startCol, endLine, endCol}
	remainder = fields[5 : len(fields)-1]
	result = fields[len(fields)-1]
	if result != PASS && result != FAIL {
		t.Fatalf("Marker is invalid: last field must be %s or %s",
			PASS, FAIL)
	}
	return
}

func parseInt(s string, t *testing.T) int {
	result, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		t.Fatalf("Marker is invalid: expecting integer, found %s", s)
	}
	return int(result)
}

// extractMarkers extracts comments of the form //<<<<<a,b,c,d,e,f,g removing
// the leading <<<<< and trimming any spaces from the left and right ends
func extractMarkers(filename string, t *testing.T) []string {
	result := []string{}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Logf("Cannot extract markers from %s -- unable to parse",
			filename)
		wd, _ := os.Getwd()
		t.Logf("Working directory is %s", wd)
		t.Fatal(err)
	}
	for _, commentGroup := range f.Comments {
		for _, comment := range commentGroup.List {
			txt := comment.Text
			if strings.Contains(txt, MARKER) {
				idx := strings.Index(txt, MARKER) + len(MARKER)
				txt = strings.TrimSpace(txt[idx:])
				result = append(result, txt)
			}
		}
	}
	return result
}
