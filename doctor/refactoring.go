package doctor

// This file defines the Refactoring interface, the RefactoringBase struct, and
// several methods common to refactorings based on RefactoringBase, including
// SetSelection, GetLog, and GetResult.

import (
	//"code.google.com/p/go.tools/go/types"
	//"code.google.com/p/go.tools/importer"
	"fmt"
	"go.tools/importer"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
)

// The Refactoring interface provides the methods common to all refactorings.
//
// Name returns a human-readable name for the refactoring, properly capitalized
// (e.g., "Rename" or "Extract Function").
//
// TODO: DOCUMENT REMAINING METHODS
type Refactoring interface {
	Name() string
	SetSelection(selection TextSelection) bool
	Run()
	GetLog() *Log
	GetResult() (*Log, EditSet)
}

type RefactoringBase struct {
	importer       *importer.Importer
	pkgInfo        *importer.PackageInfo
	file           *ast.File
	filename       string
	selectionStart token.Pos
	selectionEnd   token.Pos
	selectedNode   ast.Node
	log            *Log
	editSet        EditSet
}

// Configures a refactoring by indicating the filename in which text is
// selected and the beginning and end of the selected region.  Internally,
// this configures all of the fields in the RefactoringBase struct.
func (r *RefactoringBase) SetSelection(selection TextSelection) bool {
	r.log = NewLog()

	//r.filename = selection.filename
	filename, err := filepath.Abs(selection.filename)
	if err != nil {
		r.log.Log(FATAL_ERROR, err.Error())
		return false
	}
	r.filename = filename

	cwd, err := os.Getwd()
	if err != nil {
		r.log.Log(FATAL_ERROR, err.Error())
		return false
	}

	buildContext := build.Default
	buildContext.GOPATH = cwd

	//r.importer = importer.New(new(importer.Config))
	//r.importer = importer.New(&importer.Config{Build: &build.Default})
	fmt.Println("GOPATH is", buildContext.GOPATH)
	r.importer = importer.New(&importer.Config{Build: &buildContext})
	pkgInfo, _, err := r.importer.LoadInitialPackages([]string{r.filename})
	if err != nil {
		r.log.Log(FATAL_ERROR, err.Error())
		return false
	} else if len(pkgInfo) != 1 {
		r.log.Log(FATAL_ERROR, "Analysis error: unable to import package")
		return false
	}

	r.pkgInfo = pkgInfo[0]
	if r.pkgInfo.Err != nil {
		r.log.Log(FATAL_ERROR, r.pkgInfo.Err.Error())
		return false
	}

	if len(r.pkgInfo.Files) < 1 {
		r.log.Log(FATAL_ERROR, "Package contains no files")
		return false
	}

	r.file = r.pkgInfo.Files[0]
	if r.file == nil {
		r.log.Log(FATAL_ERROR, "Unable to parse "+selection.filename)
		return false
	}

	r.selectionStart = r.lineColToPos(selection.startLine, selection.startCol)
	r.selectionEnd = r.lineColToPos(selection.endLine, selection.endCol)

	nodes, _ := importer.PathEnclosingInterval(r.file,
		r.selectionStart, r.selectionEnd)
	r.selectedNode = nodes[0]

	r.editSet = NewEditSet()
	return true
}

//// Parses the given file, logging errors to the given log, and returning both
//// a FileSet and a File
//func (r *RefactoringBase) parse(filename string) *ast.File {
//	//f, err := parser.ParseFile(r.importer.Fset, filename, nil, parser.ParseComments)
//	fs, err := importer.ParseFiles(r.importer.Fset, ".", filename)
//	if err != nil {
//		r.log.Log(FATAL_ERROR, "Error parsing "+filename+": "+
//			err.Error())
//		return nil
//	}
//	if len(fs) != 1 {
//		r.log.Log(FATAL_ERROR, "Unable to parse " + filename)
//		return nil
//	}
//	return fs[0]
//}

// Converts a line/column position (where the first character in a file is at
// line 1, column 1) into a token.Pos
func (r *RefactoringBase) lineColToPos(line int, column int) token.Pos {
	file := r.importer.Fset.File(r.file.Pos())
	if file == nil {
		panic("file is nil")
	}
	lastLine := -1
	thisColumn := 1
	for i := 0; i < file.Size(); i++ {
		pos := file.Pos(i)
		thisLine := file.Line(pos)
		if thisLine != lastLine {
			thisColumn = 1
		} else {
			thisColumn++
		}
		if thisLine == line && thisColumn == column {
			return pos
		}
		lastLine = thisLine
	}
	return r.file.Pos()
}

func (r *RefactoringBase) checkForErrors() {
	contents, err := ioutil.ReadFile(r.filename)
	if err != nil {
		r.log.Log(ERROR, "Unable to read source file: "+err.Error())
		return
	}
	sourceFromFile := string(contents)

	string, err := r.editSet.ApplyToString(sourceFromFile)
	if err != nil {
		r.log.Log(ERROR, "Transformation produced invalid EditSet: "+
			err.Error())
		return
	}

	f, err := parser.ParseFile(r.importer.Fset, "", string, parser.ParseComments)
	if err != nil {
		fmt.Println("vvvvv")
		fmt.Println(string)
		fmt.Println("^^^^^")
		r.log.Log(ERROR, "Transformation will introduce "+
			"syntax errors: "+err.Error())
		return
	}

	// TODO: importer.CreatePackage outputs error message -- suppress
	r.pkgInfo = r.importer.CreatePackage(r.file.Name.Name, f)
	if r.pkgInfo.Err != nil {
		r.log.Log(ERROR, "Transformation will introduce semantic "+
			"errors: "+r.pkgInfo.Err.Error())
		return
	}
}

func (r *RefactoringBase) GetLog() *Log {
	return r.log
}

func (r *RefactoringBase) GetResult() (*Log, EditSet) {
	return r.log, r.editSet
}
