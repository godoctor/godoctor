package main

import (
	"fmt"
	"math"
)

func main() {
	y := 2.0
	var x float64 // <<<<< extractLocal,10,6,10,7,newVar,fail
	if math.Mod(y, 5.0) == 0 {
		x = y
		fmt.Println("divisible by 5:")
		if x > y {
			fmt.Println("")
		}
	}
}
