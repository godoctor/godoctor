// <<<<< extract,10,3,12,4,FOO,pass
package main

import "fmt"

func main() {
	xs := []float64{1, 2, 3, 4, 5}
	total := 0.0
	for i, v := range doubleXS(xs) {
		if temp, ok := tripleNum(v); ok {
			fmt.Println("##temp", temp)
		}
		fmt.Println(v, i)
		total += v
	}
	fmt.Println("Final", total)

}

func doubleXS(xs []float64) []float64 {
	var temp []float64
	for i, _ := range xs {
		temp = append(temp, xs[i]*2)
	}
	return temp
}

func tripleNum(num float64) (float64, bool) {
	return num * 3, true
}
