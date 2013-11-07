package doctor

// Implementation of the greedy LCS/SES algorithm described in
// Eugene W. Myers, "An O(ND) Difference Algorithm and Its Variations"

import (
	"fmt"
)

func Diff(a, b string) EditSet {
	result := NewEditSet()
	// result.Add(file, OffsetLenght, replacement)
	m := len(a)
	n := len(b)
	max := m + n
	v := make([]int, 2*max, 2*max)
	offset := max
	v[offset+1] = 0
	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			var x, y int
			if k == -d || k != d && v[offset+k-1] < v[offset+k+1] {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y = x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[offset+k] = x
			if x >= n && y >= m {
				// length of an SES is D
				fmt.Println(v)
				return result
			}
		}
	}
	panic("Length of SES longer than max")
}
