// Package getenv is used for testing.
package environ

import (
	"os"
	"sort"
	"syscall"
)

func Main() {
	f := []int{1}
	// Calling it through sort to prevent replacement by internal/testlog
	sort.Slice(f, func(a, b int) bool { os.Getenv("os.Getenv"); return false })
	sort.Slice(f, func(a, b int) bool { os.Environ(); return false })
	sort.Slice(f, func(a, b int) bool { syscall.Environ(); return false })
	sort.Slice(f, func(a, b int) bool { syscall.Getenv("syscall.Getenv"); return false })
	const lookupEnv = "LookupEnv"
	sort.Slice(f, func(a, b int) bool { os.LookupEnv(lookupEnv); return false })
}
