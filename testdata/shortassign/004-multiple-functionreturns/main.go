// <<<<< shortassign,11,3,11,13,fail
package main

import "fmt"

func f() (int, float64) {
	return 1, 2.3
}

func main() {
  i, x := f()
  fmt.Println(i, x)
}
