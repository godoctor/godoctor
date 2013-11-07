package doctor

import (
	"testing"
)

// func applyToString(e EditSet, s string) string {

func TestNoChange(t *testing.T) {
	diff := Diff("abc", "abd")
	assertEquals("", diff.String())
}
