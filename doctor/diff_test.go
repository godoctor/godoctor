// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Contributors: Jeff Overbey

package doctor

import (
	//"fmt"
	"math/rand"
	"testing"
	//"time"
)

var rng *rand.Rand = rand.New(rand.NewSource(99))

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

func BenchmarkDiff(bench *testing.B) {
	//	count := 5
	size := 1 * 1048576
	//	for i := 0; i < count; i++ {
	a := randomString(size)
	b := randomString(size)
	//		start := time.Now()
	bench.ResetTimer()
	/*diff :=*/ Diff("-", a, b)
	//		stop := time.Since(start)
	//		fmt.Println(stop, "to diff string of length", size)

	//		expected := b
	//		actual, err := diff.ApplyToString("-", a)
	//		if err != nil {
	//			bench.Fatal(err)
	//		} else if expected != actual {
	//			bench.Fatal("Diff failed")
	//		}
	//	}
}

func randomString(size int) string {
	s := make([]byte, size, size)
	for i := range s {
		s[i] = byte(' ' + rng.Intn('~'-' '+1))
	}
	return string(s)
}
