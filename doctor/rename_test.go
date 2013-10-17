package doctor

import (
	"bytes"
	//"fmt"
	"os"
	"path/filepath"
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
		}, t);
}

// This is old -- do not write any tests like this -- use runAllTests (above)
func TestRename_OLD__DO_NOT_WRITE_TESTS_LIKE_THIS(t *testing.T) {
	dir := filepath.Join(DIRECTORY, "001-local")
	err := os.Chdir(dir)
	if err != nil {
		t.Error(err)
	}
	LOCAL1 := "local1.go"
	r := new(RenameRefactoring)
	if !r.SetSelection(TextSelection{LOCAL1, 11, 6, 11, 6}) {
		t.Errorf("SetSelection failed")
	}
	r.SetNewName("renamed")
	//r.SetNewName("world")
	r.Run()
	log, editSet := r.GetResult()
	if log.ContainsErrors() {
		t.Errorf(log.String())
	}
	var writer bytes.Buffer
	err = editSet.ApplyToFile(LOCAL1, &writer)
	if err != nil {
		panic(err)
	}
	//fmt.Println(writer.String())
}
