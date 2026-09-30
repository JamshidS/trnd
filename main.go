package main

import (
	"fmt"
	"os"
	"runtime/debug"
)

// version is set at build time using -ldflags "-X main.version= ...."
var version = ""

func main() {
	fmt.Println("Version:", buildVersion())
}

func buildVersion() string {
	if version != "" {
		return version
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return "dev"
}