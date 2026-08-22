package main

import (
	"os"

	"github.com/charleszardd/daegsa/internal/cli"
)

func main() {
	code := cli.Execute()
	os.Exit(int(code))
}
