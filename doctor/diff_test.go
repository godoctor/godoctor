package doctor

import (
	"testing"
)

// func applyToString(e EditSet, s string) string {

func testDiff(a, b string, t *testing.T) {
	diff := Diff("-", a, b)
	actual, err := diff.ApplyToString("-", a)
	failIfError(err, t)
	expected := b
	assertEquals(expected, actual, t)
}

func TestMyersPaperExample(t *testing.T) {
	testDiff("ABCABBA", "CBABAC", t)
}

func TestEmptyCases(t *testing.T) {
	testDiff("", "", t)
	testDiff("", "abcdefg", t)
	testDiff("abcdefg", "", t)
}

func TestNoChange(t *testing.T) {
	testDiff("a", "a", t)
	testDiff("a\nbcd", "a\nbcd", t)
}

func TestAdditionalDiffs(t *testing.T) {
	testDiff("abcdefg", "abcfg", t)
	testDiff("abcdefg", "defg", t)
	testDiff("abcdefg", "abcd", t)
	testDiff("abcdefg", "ag", t)
	testDiff("abcdefg", "bcd", t)
	testDiff("abcdefg", "bdf", t)
	testDiff("abc", "defg", t)
	testDiff("abcdefg", "xyz", t)
}
