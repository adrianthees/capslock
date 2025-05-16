// Package getenv is used for testing.
package getenv

import (
	"os"
	"sort"
)

func Foo() {
	f := []int{1}
	// Calling it through sort to prevent replacement by internal/testlog
	sort.Slice(f, func(a, b int) bool { os.Getenv("FOO"); return false })
}
