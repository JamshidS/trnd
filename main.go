package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/jamshids/trnd/internal/cli"
	"github.com/jamshids/trnd/internal/style"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = ""

func main() {
	if err := cli.Execute(buildVersion()); err != nil {
		fmt.Fprintln(os.Stderr, style.Error.Render("Error: ")+err.Error())
		os.Exit(1)
	}
}

// buildVersion falls back to the module version for `go install` builds.
func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
