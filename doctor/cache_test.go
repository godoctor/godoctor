package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var c Cache
var paths []string

func BenchmarkLoad(b *testing.B) {
	b.ReportAllocs()
	c = make(Cache)
	err := filepath.Walk("../", func(p string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		paths = append(paths, p)
		return c.LoadFile(p)
	})
	if err != nil {
		fmt.Println(err)
	}
}

func BenchmarkGet(b *testing.B) {
	for _, p := range paths {
		x, err := c.Get(p)
		if err != nil {
			fmt.Println(err)
		}
		_ = x
	}
}
