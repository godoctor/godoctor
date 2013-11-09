package main //<<<<<fiximports,1,1,1,1,pass

import (
	f "bogus"
	p "my/bogus"
	"another/bogus"
)

func main() {
	bogus.do(f.Println(p.PkgFunc()))
}
