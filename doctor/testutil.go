package doctor

// This file defines utility functions used exclusively by the testing
// infrastructure.

import (
	"bytes"
	"fmt"
	"go/token"
	"go/parser"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const MARKER = "<<<<<"
const PASS = "pass"
const FAIL_SELECTION = "fail-selection"
const FAIL_CONFIGURE = "fail-configure"
const FAIL = "fail"

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		t.Errorf("Expected: %s Actual: %s", expected, actual)
	}
}

// assertError is a utility method for unit tests that marks a function as
// having failed if the given string does not begin with "ERROR: "
func assertError(result string, t *testing.T) {
	if !strings.HasPrefix(result, "ERROR: ") {
		t.Errorf("Expected error; actual: \"%s\"", result)
	}
}

// runAllTests is a utility method that runs a set of refactoring tests
// based on markers in all of the files in subdirectories of a given directory
func runAllTests(directory string,
		r Refactoring, configure func ([]string) bool,
		t *testing.T) {

	testDirs, err := ioutil.ReadDir(directory)
	if err != nil {
		t.Errorf("Unable to read test directory %s", directory)
		t.FailNow()
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	defer os.Chdir(cwd)

	for _, testDirInfo := range testDirs {
		testDirPath := filepath.Join(directory, testDirInfo.Name())

		files, err := ioutil.ReadDir(testDirPath)
		if err != nil {
			t.Errorf("Unable to read directory %s", directory)
			t.FailNow()
		}

		err = os.Chdir(testDirPath)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}

		runTestsInFiles(files, r, configure, t)
		
		os.Chdir(cwd)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	}
}

func runTestsInFiles(files []os.FileInfo,
		r Refactoring, configure func ([]string) bool,
		t *testing.T) {
	markers := make(map[string][]string)
	for _, fileInfo := range files {
		filename := fileInfo.Name()
		if strings.HasSuffix(filename, ".go") {
			markers[filename] = extractMarkers(filename, t)
		}
	}

	for filename, markersInFile := range markers {
		for _, marker := range markersInFile {
			if filename == "main.go" {
				cmd := exec.Command("go", "run", "main.go")
				_, err := cmd.Output()
				if err != nil {
					fmt.Println("go run main.go failed:")
					t.Error(err)
					t.FailNow()
				}
			}
			runRefactoring(filename, marker, r, configure, t)
			// TODO: Compare output after refactoring
		}
	}
}

func runRefactoring(filename string, marker string, 
		r Refactoring, configure func ([]string) bool,
		t *testing.T) {

	selection, remainder, result := splitMarker(filename, marker, t)
	shouldPass := (result == PASS)
	name := r.Name()

	fmt.Println("Running", name, "on", filename, selection)

	ok := r.SetSelection(selection)
	log := r.GetLog()
	if result == FAIL_SELECTION && !ok {
		return // We expected SetSelection to fail -- good
	} else if shouldPass && (!ok || log.ContainsErrors()) {
		t.Errorf("SetSelection produced unexpected errors")
		fmt.Println(log)
		t.FailNow()
	}

	ok = configure(remainder)
	log = r.GetLog()
	if result == FAIL_CONFIGURE && !ok {
		return // We expected configuration to fail -- good
	} else if shouldPass && (!ok || log.ContainsErrors()) {
		t.Errorf("Refactoring configuration failed")
		fmt.Println(log)
		t.FailNow()
	}

	r.Run()
	log, edits := r.GetResult()
	if shouldPass && log.ContainsErrors() {
		fmt.Println(log)
		t.Errorf("Refactoring produced unexpected errors")
		t.FailNow()
	}

	var output bytes.Buffer
	err := edits.ApplyToFile(filename, &output)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if shouldPass {
		checkResult(filename, output.String(), t)
	}
}

func checkResult(filename string, actualOutput string, t *testing.T) {
	bytes, err := ioutil.ReadFile(filename + "lden")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	expectedOutput := string(bytes)

	if actualOutput != expectedOutput {
		fmt.Println(">>>>> Output does not match .golden file")
		fmt.Println("EXPECTED OUTPUT")
		fmt.Println("vvvvvvvvvvvvvvv")
		fmt.Println(expectedOutput)
		fmt.Println("^^^^^^^^^^^^^^^")
		fmt.Println("ACTUAL OUTPUT")
		fmt.Println("vvvvvvvvvvvvv")
		fmt.Println(actualOutput)
		fmt.Println("^^^^^^^^^^^^^")
		t.Errorf("Refactoring test failed - %s", filename)
		t.FailNow()
	}
}

func splitMarker(filename string, marker string, t *testing.T) (
		selection TextSelection, remainder []string, result string) {
	fields := strings.Split(marker, ",")
	if (len(fields) < 5) {
		t.Errorf("Marker is invalid (must contain >= 5 fields): %s",
			marker)
		t.FailNow()
	}
	startLine := parseInt(fields[0], t)
	startCol := parseInt(fields[1], t)
	endLine := parseInt(fields[2], t)
	endCol := parseInt(fields[3], t)
	selection = TextSelection{filename,
		startLine, startCol, endLine, endCol}
	remainder = fields[4:len(fields)-1]
	result = fields[len(fields)-1]
	if result != PASS && result != FAIL {
		t.Errorf("Marker is invalid: last field must be one of: " +
			"%s, %s, %s, or %s",
			PASS, FAIL_SELECTION, FAIL_CONFIGURE, FAIL)
		t.FailNow()
	}
	return
}

func parseInt(s string, t *testing.T) int {
	result, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		t.Errorf("Marker is invalid: expecting integer, found %s", s)
		t.FailNow()
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
		t.Errorf("Error parsing %s", filename)
		t.FailNow()
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
