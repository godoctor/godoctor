package doctor

import (
	"bytes"
	"fmt"
	//"go/ast"
	//"go/parser"
	//"go/token"
	"os"
	"testing"
)

func TestRename1(t *testing.T) {
	DIR := "testdata/rename"
	err := os.Chdir(DIR)
	if err != nil {
		t.Error(err)
	}
	LOCAL1 := "local1.go"
	r := new(RenameRefactoring)
	if !r.SetSelection(TextSelection{LOCAL1, 11, 6, 11, 6}) {
		t.Errorf("SetSelection failed")
	}
	r.SetNewName("renamed")
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
	fmt.Println(writer.String())
}
