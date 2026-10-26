//go:build e2e

package e2e

import (
	"flag"
	"os"
	"testing"
)

var localMode = flag.Bool("local", false, "run tests against a local server instead of containers")

func TestMain(m *testing.M) {
	flag.Parse()

	SetupSharedEnvironment(*localMode)

	// os.Exit skips defers, so tear down explicitly before exiting.
	code := m.Run()
	TeardownSharedEnvironment()
	os.Exit(code)
}
