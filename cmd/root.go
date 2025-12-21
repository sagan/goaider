package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

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

var (
	FlagEnv []string
)

func init() {
	RootCmd.PersistentFlags().StringArrayVarP(&FlagEnv, "env", "e", nil,
		`Set env. "name=value" format. Set be set multiple times`)
}

func Execute() {
	cobra.OnInitialize(sync.OnceFunc(func() {
		for _, env := range FlagEnv {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			} else {
				fmt.Fprintf(os.Stderr, "Invalid env format: %s. Expected name=value\n", env)
				os.Exit(1)
			}
		}
	}))
	if err := RootCmd.Execute(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
