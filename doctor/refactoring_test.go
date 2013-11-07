package doctor

// Tests for all refactorings.  The testdata directory is structured as such:
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
// refactorings are expected to succeed or fail.  If a refactoring is
// expected to succeed, the resulting file is compared against a .golden file
// in the same directory.
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
//                     package-name/
//                         package-file.go
//

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var filterFlag = flag.String("filter", "", "Only tests from directories containing this substring will be run")

const directory = "../testdata/"

func TestRefactorings(t *testing.T) {
	testDirs, err := ioutil.ReadDir(directory)
	failIfError(err, t)
	for _, testDirInfo := range testDirs {
		if testDirInfo.IsDir() {
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
			//desc := filepath.Join(testDirInfo.Name(), subDirInfo.Name())
			if strings.Contains(subDirPath, *filterFlag) {
				//fmt.Printf("********** %s **********\n", desc)
				RunAllTestsInDirectory(subDirPath, t)
			} else {
				//fmt.Printf("Skipping directory %s (filtered)\n", desc)
			}
		}
	}
}
