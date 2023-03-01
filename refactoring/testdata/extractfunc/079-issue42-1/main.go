package main

import (
	"fmt"
	"strings"
)

type Dummy struct{}

func main() {
	inputString := "bob"
	listOfWork := []string{"a", "b", "c"}
	var dm Dummy
	inputString = dm.work(inputString, listOfWork)
	fmt.Println("done something", inputString)
}

func (dm Dummy) work(tmp string, listOfWork []string) (outputString string) {
	inputString := tmp + " Suffix"
	wrkDone := false
	// This if can't seem to be extracted with any success
	if !wrkDone && strings.Contains(inputString, "Suffix") { // <<<<<extract,22,2,24,2,extracted,pass
		wrkDone = true
	}
	if wrkDone {
		fmt.Println("Got something", inputString)
	}
	outputString = "steve"
	return
}
