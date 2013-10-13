package doctor

// This file defines utility functions used exclusively by the testing
// infrastructure.

import (
	"strings"
	"testing"
)

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
