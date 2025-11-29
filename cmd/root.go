package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/goaider/version"
)

var RootCmd = &cobra.Command{
	Use:   "goaider",
	Short: "goaider " + version.Version,
	Long: `goaider ` + version.Version + "." + `
A CLI aider tool for AIGC.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
