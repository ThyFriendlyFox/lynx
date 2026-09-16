// Command lynx keeps a folder the same on every computer that holds it.
package main

import (
	"os"

	"github.com/thyfriendlyfox/lynx/internal/cli"
)

// version is set with -ldflags at build time.
var version = "dev"

func main() {
	cli.Version = version
	os.Exit(cli.Main(os.Args))
}
