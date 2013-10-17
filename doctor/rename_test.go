package doctor

import (
	"testing"
)

const DIRECTORY = "../testdata/rename"

func TestRename(t *testing.T) {
	rename := new(RenameRefactoring)
	runAllTests(DIRECTORY, rename,
		func(args []string) bool {
			if len(args) != 1 {
				t.Errorf("Marker is missing new name")
			}
			rename.SetNewName(args[0])
			return true
		}, t)
}
