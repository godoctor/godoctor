// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains an implementation of the greedy longest common
// subsequence/shortest edit script (LCS/SES) algorithm described in
// Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations"

// Contributors: Jeff Overbey

package doctor

func Diff(filename, a, b string) EditSet {
	n := len(a)
	m := len(b)
	max := m + n
	if n == 0 && m == 0 {
		return NewEditSet()
	} else if n == 0 {
		result := NewEditSet()
		result.Add(filename, OffsetLength{0, 0}, b)
		return result
	} else if m == 0 {
		result := NewEditSet()
		result.Add(filename, OffsetLength{0, len(a)}, "")
		return result
	}
	vs := make([][]int, 0, max)
	v := make([]int, 2*max, 2*max)
	offset := max
	v[offset+1] = 0
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x, y int
			var vert bool
			if k == -d || k != d &&
				abs(v[offset+k-1]) < abs(v[offset+k+1]) {
				x = abs(v[offset+k+1])
				vert = false
			} else {
				x = abs(v[offset+k-1]) + 1
				vert = true
			}
			y = x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			if vert {
				v[offset+k] = -x
			} else {
				v[offset+k] = x
			}
			if x >= n && y >= m {
				// length of SES is D
				vs = append(vs, v)
				return constructEditSet(filename, a, b, vs)
			}
		}
		v_copy := make([]int, len(v))
		copy(v_copy, v)
		vs = append(vs, v_copy)
	}
	panic("Length of SES longer than max")
}

func abs(n int) int {
	if n < 0 {
		return -n
	} else {
		return n
	}
}

func constructEditSet(filename, a, b string, vs [][]int) EditSet {
	n := len(a)
	m := len(b)
	max := m + n
	offset := max
	result := NewEditSet()
	k := n - m
	for len(vs) > 1 {
		v := vs[len(vs)-1]
		v_k := v[offset+k]
		x := abs(v_k)
		y := x - k

		vs = vs[:len(vs)-1]
		v = vs[len(vs)-1]
		if v_k > 0 {
			k++
		} else {
			k--
		}
		next_v_k := v[offset+k]
		next_x := abs(next_v_k)
		next_y := next_x - k

		if v_k > 0 {
			// Insert
			charsToCopy := y - next_y - 1
			insertOffset := x - charsToCopy
			ol := OffsetLength{insertOffset, 0}
			copyOffset := y - charsToCopy - 1
			replaceWith := b[copyOffset : copyOffset+1]
			result.Add(filename, ol, replaceWith)
		} else {
			// Delete
			charsToCopy := x - next_x - 1
			deleteOffset := x - charsToCopy - 1
			ol := OffsetLength{deleteOffset, 1}
			replaceWith := ""
			result.Add(filename, ol, replaceWith)
		}
	}
	return result
}
