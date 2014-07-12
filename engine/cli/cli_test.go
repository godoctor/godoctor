// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cli_test

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"

	"golang-refactoring.org/go-doctor/engine/cli"
)

const (
	hello = `package main
import "fmt"
var こんにちはmsg string = "Hello, package"
func main() {
	fmt.Println(こんにちはmsg)
}`
	pos = "-pos=3,5:3,5" // position to rename (msg variable)

	diff = `diff -u /dev/stdin /dev/stdout
--- /dev/stdin
+++ /dev/stdout
@@ -1,6 +1,6 @@
 package main
 import "fmt"
-var こんにちはmsg string = "Hello, package"
+var renamedネーム string = "Hello, package"
 func main() {
-	fmt.Println(こんにちはmsg)
+	fmt.Println(renamedネーム)
 }`

	complete = `@@@@@ /dev/stdin @@@@@ 119 @@@@@
package main
import "fmt"
var renamedネーム string = "Hello, package"
func main() {
	fmt.Println(renamedネーム)
}
`
)

func runCLI(stdin string, args ...string) (exit int, stdout string, stderr string) {
	args = append(args, "godoctor")
	copy(args[1:], args[0:len(args)-1])
	args[0] = "godoctor"

	var stdoutBuf, stderrBuf bytes.Buffer
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	exit = cli.Run(strings.NewReader(stdin), &stdoutBuf, &stderrBuf, args)
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()
	return
}

func TestNoArgsNoInput(t *testing.T) {
	exit, stdout, stderr := runCLI("")
	if exit != 2 || stdout != "" ||
		!strings.Contains(stderr, "Usage: godoctor ") {
		t.Fatal("No args, no input expected usage string with exit 2")
	}
}

func TestHelp(t *testing.T) {
	for _, helpFlag := range []string{"-help", "--help", "help"} {
		exit, stdout, stderr := runCLI("", helpFlag)
		if exit != 2 || stdout != "" ||
			!strings.Contains(stderr, "Usage: godoctor ") {
			t.Fatalf("%s expected usage string with exit 2", helpFlag)
		}
	}
}

func TestInvalidFlag(t *testing.T) {
	exit, stdout, stderr := runCLI("", "-somethinginvalid")
	if exit != 1 || stdout != "" || stderr == "" {
		t.Fatal("Invalid flag expected exit 1")
	}

	exit, stdout, stderr = runCLI("", "-complete=thisshouldbeaboolean")
	if exit != 1 || stdout != "" || stderr == "" {
		t.Fatal("Invalid flag expected exit 1")
	}
}

func TestList(t *testing.T) {
	exit, stdout, stderr := runCLI("", "-list")
	if exit != 0 || stdout != "" || !strings.Contains(stderr, "rename") {
		t.Fatalf("-list expected refactoring list with exit 0")
	}

	for _, flag := range []string{"-w", "-complete", "-json"} {
		exit, stdout, stderr = runCLI("", flag, "-list")
		if exit != 1 || stdout != "" || !strings.Contains(stderr,
			"-list flag cannot be used with") {
			t.Fatalf("-list should fail and exit 1 if used with %s", flag)
		}
	}
}

func TestInvalidCombos(t *testing.T) {
	invalid := [][]string{
		// complete file json list pos scope verbose write
		[]string{"-complete", "-json"},
		[]string{"-complete", "-list"},
		[]string{"-complete", "-w"},
		[]string{"-file=-", "-json"},
		[]string{"-json", "-list"},
		[]string{"-json", "-pos=1,1:1,1"},
		[]string{"-json", "-scope=code.google.com/p/go.tools"},
		[]string{"-json", "-v"},
		[]string{"-json", "-w"},
		[]string{"-list", "-v"},
		[]string{"-list", "-w"},
		[]string{"-list", "somearg"},
	}
	for _, flags := range invalid {
		exit, stdout, stderr := runCLI("", flags...)
		if exit != 1 || stdout != "" || !strings.Contains(stderr, "cannot") {
			t.Fatalf("Expected failure and exit 1 if using %s",
				strings.Join(flags, " "))
		}
	}
}

/* FIXME: Enable this after JSON does not fix output to os.Stdout
func TestJSONSmoke(t *testing.T) {
	jsonArg := `[{"command":"list","quality":"in_development"}]`
	exit, stdout, stderr := runCLI("", "-json", jsonArg)
	if exit != 0 || stderr != "" {
		t.Fatalf("-json with argument failed")
	}
	reply := map[string]interface{}{}
	json.Unmarshal([]byte(stdout), &reply)
	if reply["reply"] != "OK" ||
		len(reply["transformations"].([]interface{})) !=
			len(engine.AllRefactorings()) {
		t.Fatalf("JSON expected OK reply, %d refactorings, got %v",
			len(engine.AllRefactorings()), reply["transformations"])
	}
}
*/

func TestInvalidRefactoring(t *testing.T) {
	exit, stdout, stderr := runCLI("", "InvalidRefactoringName")
	if exit != 1 || stdout != "" ||
		!strings.Contains(stderr, "There is no refactoring named") {
		t.Fatal("Invalid refactoring expected exit 1")
	}
}

func TestRefactoringUsage(t *testing.T) {
	exit, stdout, stderr := runCLI("", "rename")
	if exit != 2 || stdout != "" || !strings.Contains(stderr, "Usage:") {
		t.Fatal("\"doctor rename\" expected usage info with exit 2")
	}
}

func TestRenameDiff(t *testing.T) {
	exit, stdout, _ := runCLI(hello, pos, "rename", "renamedネーム")
	if exit != 0 {
		t.Fatal("Rename expected clean exit")
	}
	if stdout != diff {
		t.Fatalf("Output did not match expected diff:\n%s", stdout)
	}
}

func TestRenameComplete(t *testing.T) {
	exit, stdout, _ := runCLI(hello, pos, "-complete", "rename", "renamedネーム")
	if exit != 0 {
		t.Fatal("Rename expected clean exit")
	}
	if stdout != complete {
		t.Fatalf("Output did not match expected output:\n%s", stdout)
	}
}

func TestRenameInvalidPos(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-pos=1000,", "rename", "x")
	if exit != 1 || stderr == "" {
		t.Fatal("Rename with invalid position expected error exit 1")
	}
	if stdout != "" {
		t.Fatalf("Rename with invalid -pos should not have output")
	}
}

func TestRenamePosOutOfRange(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-pos=1000,1:1000,1", "rename", "x")
	if exit != 0 || stderr == "" {
		t.Fatal("Rename position out of range expected clean exit")
	}
	if stdout != "" {
		t.Fatalf("Rename position out of range should not have output")
	}
}

func TestRenameInvalidScope(t *testing.T) {
	exit, stdout, stderr := runCLI(hello, "-scope=invalidScope", "null", "false")
	if exit != 0 || stderr == "" {
		t.Fatal("Rename with invalid scope should produce clean exit")
	}
	if stdout != "" {
		t.Fatalf("Rename with invalid scope should not have output")
	}
}
