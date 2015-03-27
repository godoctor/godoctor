package main

import "fmt"

func main() {
	data := [...]int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	for index := 0; index < len(data); index++ {
		switch {
		case data[index] == 5:
			fmt.Printf("this is a test: 5")
		case data[index] == 7:
			fmt.Printf("this is a test: 7")
		case data[index] > 9:
			switch {
			case data[index] > 10:
				switch {
				case data[index] > 13: // <<<<< extractLocal,18,10,18,20,newVar,pass
					fmt.Printf("this is a test: > 13")
				}
			default:
			}
		default:
		}
	}
}
