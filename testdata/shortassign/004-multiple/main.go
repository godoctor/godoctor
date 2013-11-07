package main

import "fmt"

func f() (int, float64) {
	return 1, 2.3
}

func main() {
  i, x := f()    // <<<<< shortassign,10,3,10,13,fail
  fmt.Println(i, x)
}
