package refactoring_test

import "github.com/godoctor/godoctor/refactoring/testutil"
import "testing"

const directory = "testdata/"

func TestRefactorings(t *testing.T) {
	testutil.TestRefactorings(directory, t)
}
