package doctor

// This file defines utility functions used exclusively by the testing
// infrastructure.

import (
	"bytes"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Modify this to temporarily filter out unwanted tests
func shouldRunTest(dirname string) bool {
	return strings.HasPrefix(dirname, "002-")
	//return true
}

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
	r Refactoring, configure func([]string) bool,
	t *testing.T) {
	fmt.Println("********************************************************************************")
	fmt.Println("*", strings.ToUpper(r.Name()))
	fmt.Println("********************************************************************************")

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
		if !shouldRunTest(testDirInfo.Name()) {
			fmt.Println("Skipping", testDirInfo.Name())
		} else {
			testDirPath := filepath.Join(directory, testDirInfo.Name())
			err = os.Chdir(testDirPath)
			failIfError(err, t)

			absolutePath, err := filepath.Abs(".")
			failIfError(err, t)

			files, err := recursiveReadDir(".")
			failIfError(err, t)

			err = os.Setenv("GOPATH", absolutePath)
			failIfError(err, t)

			runTestsInFiles(files, r, configure, t)

			os.Chdir(cwd)
			failIfError(err, t)
		}
	}
}

// Assumes no duplication or circularity due to symbolic links
func recursiveReadDir(path string) ([]string, error) {
	result := []string{}

	fileInfos, err := ioutil.ReadDir(path)
	if err != nil { return []string{}, err }

	for _, fi := range fileInfos {
		if fi.IsDir() {
			current := result
			rest, err := recursiveReadDir(filepath.Join(path, fi.Name()))
			if err != nil { return []string{}, err }

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
		t.Error(err)
		t.FailNow()
	}
}

func runTestsInFiles(files []string,
	r Refactoring, configure func([]string) bool,
	t *testing.T) {
	markers := make(map[string][]string)
	for _, path := range files {
		if strings.HasSuffix(path, ".go") {
			markers[path] = extractMarkers(path, t)
		}
	}

	if len(markers) == 0 {
		pwd, _ := os.Getwd()
		pwd = filepath.Base(pwd)
		t.Errorf("No <<<<< markers found in any files in %s", pwd)
	}

	for path, markersInFile := range markers {
		for _, marker := range markersInFile {
			runRefactoring(path, marker, r, configure, t)
		}
	}
}

func runRefactoring(filename string, marker string,
	r Refactoring, configure func([]string) bool,
	t *testing.T) {

	selection, remainder, result := splitMarker(filename, marker, t)
	shouldPass := (result == PASS)
	name := r.Name()

	cwd, _ := os.Getwd()
	cwd = filepath.Base(cwd)
	relativePath := filepath.Join(cwd, filename)
	fmt.Println("Running", name, "on", relativePath, selection)

	if filename == "main.go" {
		cmd := exec.Command("go", "run", "main.go")
		_, err := cmd.Output()
		if err != nil {
			fmt.Println("go run main.go failed:")
			t.Error(err)
			t.FailNow()
		}
	}

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
	fmt.Println("Comparing", filename, "to", filename+"lden")

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
	if len(fields) < 5 {
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
	remainder = fields[4 : len(fields)-1]
	result = fields[len(fields)-1]
	if result != PASS && result != FAIL {
		t.Errorf("Marker is invalid: last field must be one of: "+
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
		t.Errorf("Cannot extract markers from %s -- unable to parse",
			filename)
		wd, _ := os.Getwd()
		t.Logf("Working directory is %s", wd)
		t.Log(err)
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
