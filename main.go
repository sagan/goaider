package main

import (
	"github.com/sagan/goaider/cmd"
	_ "github.com/sagan/goaider/cmd/all"
	"github.com/sagan/goaider/features/osfeature"
)

func main() {
	osfeature.Init()
	cmd.Execute()
}
