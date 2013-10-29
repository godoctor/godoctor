package doctor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Entrypoint for the refactoring engine.  This package enumerates the
// available refactorings and provides the a short name for each refactoring
// (which is used by tests, among other things).

var refactorings map[string]Refactoring

func init() {
	refactorings = map[string]Refactoring{
		"null":   new(NullRefactoring),
		"rename": new(RenameRefactoring),
	}
}

func GetAllRefactorings() map[string]Refactoring {
	return refactorings
}

func GetRefactoring(shortName string) Refactoring {
	return refactorings[shortName]
}

//TODO is this what util is for?
//e.g. 302,6
func parseLineCol(linecol string) (int, int) {
	lc := strings.Split(linecol, ",")
	if l, err := strconv.ParseInt(lc[0], 10, 32); err == nil {
		if c, err := strconv.ParseInt(lc[1], 10, 32); err == nil {
			return int(l), int(c)
		}
	}

	return -1, -1
}

//pos=filename:3,6:3,9
func parsePositionToTextSelection(pos string) (t TextSelection, err error) {
	args := strings.Split(pos, ":")

	if len(args) < 3 {
		err = fmt.Errorf("invalid -pos")
		return
	}

	sl, sc := parseLineCol(args[1])
	el, ec := parseLineCol(args[2])

	if sl < 0 || sc < 0 || el < 0 || ec < 0 {
		err = fmt.Errorf("invalid -pos line, col")
		return
	}

	t = TextSelection{filename: args[0],
		startLine: sl, startCol: sc, endLine: el, endCol: ec}

	return
}

//TODO (reed / josh) scope here?
//
//This will do all of the configuration and execution for
//a refactoring (@op), returning the edits to be made and log.
//For use with the CLI, but have at it.
//
func Query(args []string, op string, pos string, scope string) (*Log, EditSet) {
	r := GetRefactoring(op)

	if r == nil {
		fmt.Errorf("Invalid refactoring")
		os.Exit(2)
	}

	ts, err := parsePositionToTextSelection(pos)

	//TODO: ICK
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	r.SetSelection(ts)
	r.Configure(args)
	r.Run()
	return r.GetResult()
}
