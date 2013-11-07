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

const MARKER = "<<<<<"
const PASS = "pass"
const FAIL_SELECTION = "fail-selection"
const FAIL_CONFIGURE = "fail-configure"
const FAIL = "fail"

// assertEquals is a utility method for unit tests that marks a function as
// having failed if expected != actual
func assertEquals(expected string, actual string, t *testing.T) {
	if expected != actual {
		t.Fatalf("Expected: %s Actual: %s", expected, actual)
	}
}

// assertError is a utility method for unit tests that marks a function as
// having failed if the given string does not begin with "ERROR: "
func assertError(result string, t *testing.T) {
	if !strings.HasPrefix(result, "ERROR: ") {
		t.Fatalf("Expected error; actual: \"%s\"", result)
	}
}

// RunAllTests is a utility method that runs a set of refactoring tests
// based on markers in all of the files in subdirectories of a given directory
func RunAllTests() {
	panic("RunAllTests disabled")
}

// RunAllTests is a utility method that runs a set of refactoring tests
// based on markers in all of the files in subdirectories of a given directory
func RunAllTestsInDirectory(directory string, t *testing.T) {
	files, err := recursiveReadDir(directory)
	failIfError(err, t)

	absolutePath, err := filepath.Abs(directory)
	failIfError(err, t)
	err = os.Setenv("GOPATH", absolutePath)
	failIfError(err, t)

	runTestsInFiles(files, t)
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

func runTestsInFiles(files []string, t *testing.T) {
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
			runRefactoring(path, marker, t)
		}
	}
}

func runRefactoring(filename string, marker string, t *testing.T) {
	refac, selection, remainder, result := splitMarker(filename, marker, t)

	r := GetRefactoring(refac)
	if r == nil {
		t.Fatalf("There is no refactoring named %s (from marker %s)", refac, marker)
	}

	shouldPass := (result == PASS)
	name := r.Name()

	cwd, _ := os.Getwd()
	cwd = filepath.Base(cwd)
	//relativePath := filepath.Join(cwd, filename)
	fmt.Println(name, selection.String())

	if filename == "main.go" {
		cmd := exec.Command("go", "run", "main.go")
		_, err := cmd.Output()
		if err != nil {
			t.Logf("go run main.go failed:")
			t.Fatal(err)
		}
	}

	ok := r.SetSelection(selection)
	rlog := r.GetLog()
	if result == FAIL_SELECTION && !ok {
		return // We expected SetSelection to fail -- good
	} else if shouldPass && (!ok || rlog.ContainsNonInitialErrors()) {
		t.Logf("SetSelection produced unexpected errors")
		t.Fatal(rlog)
	}

	ok = r.Configure(remainder)
	rlog = r.GetLog()
	if result == FAIL_CONFIGURE && !ok {
		return // We expected configuration to fail -- good
	} else if shouldPass && (!ok || rlog.ContainsNonInitialErrors()) {
		t.Log("Refactoring configuration failed")
		t.Fatal(rlog)
	}

	r.Run()
	rlog, edits := r.GetResult()
	if shouldPass && rlog.ContainsErrors() {
		t.Log(rlog)
		t.Fatalf("Refactoring produced unexpected errors")
	}

	var output bytes.Buffer
	err := edits.ApplyToFile(filename, &output)
	if err != nil {
		t.Fatal(err)
	}
	if shouldPass {
		checkResult(filename, output.String(), t)
	}
}

func checkResult(filename string, actualOutput string, t *testing.T) {
	bytes, err := ioutil.ReadFile(filename + "lden")
	if err != nil {
		t.Fatal(err)
	}
	expectedOutput := string(bytes)

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
		t.Fatalf("Refactoring test failed - %s", filename)
	}
}

func splitMarker(filename string, marker string, t *testing.T) (refac string, selection TextSelection, remainder []string, result string) {
	fields := strings.Split(marker, ",")
	if len(fields) < 6 {
		t.Fatalf("Marker is invalid (must contain >= 5 fields): %s", marker)
	}
	refac = fields[0]
	startLine := parseInt(fields[1], t)
	startCol := parseInt(fields[2], t)
	endLine := parseInt(fields[3], t)
	endCol := parseInt(fields[4], t)
	selection = TextSelection{filename,
		startLine, startCol, endLine, endCol}
	remainder = fields[5 : len(fields)-1]
	result = fields[len(fields)-1]
	if result != PASS && result != FAIL {
		t.Fatalf("Marker is invalid: last field must be one of: "+
			"%s, %s, %s, or %s",
			PASS, FAIL_SELECTION, FAIL_CONFIGURE, FAIL)
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
