// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

package doctor

import (
	"math/rand"
	"strings"
	"testing"
)

var rng *rand.Rand = rand.New(rand.NewSource(99))

func testDiff(a, b string, t *testing.T) {
	diff := Diff(strings.Split(a, ""), strings.Split(b, ""))
	actual, err := ApplyToString(diff, a)
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
