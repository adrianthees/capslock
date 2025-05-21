// Package getenv is used for testing.
package getenv

import (
	"os"
	"sort"
)

func Foo(env string) string {
	// Variable
	return env
}

func Bar(env string) {
	// Variable
	sort.Slice([]int{1}, func(a, b int) bool { os.Getenv(env); return false })
}

func Main() {
	f := []int{1}
	// Calling it through sort to prevent replacement by internal/testlog
	sort.Slice(f, func(a, b int) bool { os.Getenv("literal"); return false }) // literal

	const foo = "const"
	sort.Slice(f, func(a, b int) bool { os.Getenv(foo); return false }) // const

	sort.Slice(f, func(a, b int) bool { os.Getenv(Foo("functionreturn")); return false }) // function return

	var bar = "variable"
	bar = "variable2"
	sort.Slice(f, func(a, b int) bool { os.Getenv(bar); return false }) // variable

}
