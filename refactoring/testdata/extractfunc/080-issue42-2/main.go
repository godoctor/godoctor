package main

import (
	"fmt"
	"strings"
)

type Dummy struct{}

func main() {
	inputString := "bob"
	listOfWork := []string{"a", "b", "c"}
	var workDone bool
	var dum Dummy
	inputString, workDone = dum.work(listOfWork, inputString)

	if workDone {
		fmt.Println("done something", inputString)
	}
}

func (dm Dummy) work(listOfWork []string, inputString string) (string, bool) {
	var workDone bool
	// This for loop can't seem to be extracted with any success
	for _, wrk := range listOfWork { // <<<<<extract,25,2,30,2,extracted,pass
		for strings.Contains(inputString, wrk) {
			inputString = strings.Replace(inputString, wrk, ".", -1)
			workDone = true
		}
	}

	return inputString, workDone
}
