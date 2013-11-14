package mypackage
//Test for renaming an exported function name 
func MyFunction(n int) int {             // <<<<< rename,3,7,3,7,Xyz,pass
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}
