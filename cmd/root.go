package cmd

import (
	"fmt"
	"os"

	"github.com/sagan/goaider/version"
	"github.com/spf13/cobra"
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
