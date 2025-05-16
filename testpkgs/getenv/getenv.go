// Package getenv is used for testing.
package getenv

import (
	"os"
)

func Foo() {
	_ = os.Getenv("FOO")
}
