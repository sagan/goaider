package join

import (
	"fmt"
	"os"
	"strings"

	"github.com/sagan/goaider/cmd/csv"
	"github.com/sagan/goaider/util"
	"github.com/spf13/cobra"
)

var joinCmd = &cobra.Command{
	Use:   "join <left.csv> <right.csv>",
	Short: "merge (left join) two csv",
	Long:  `merge (left join) two csv.`,
	Args:  cobra.ExactArgs(2),
	RunE:  join,
}

var (
	flagForce       bool   // force overwrite existing file
	flagOn          string // join on field, can be "left_field:right_field" format
	flagLeftPrefix  string // Prefix columns of left csv with this string + "_"
	flagRightPrefix string // Prefix columns of right csv with this string + "_"
	flagOutput      string // output file, set to "-" to output to stdout
	flagFullJoin    bool   // do a full join instead of left join
)

func join(cmd *cobra.Command, args []string) (err error) {
	argLeftCsv := args[0]
	argRightCsv := args[1]
	leftOn := ""
	rightOn := ""
	if strings.Contains(flagOn, ":") {
		leftOn, rightOn, _ = strings.Cut(flagOn, ":")
	} else {
		leftOn = flagOn
		rightOn = flagOn
	}
	leftOn = strings.TrimSpace(leftOn)
	rightOn = strings.TrimSpace(rightOn)
	if leftOn == "" || rightOn == "" {
		return fmt.Errorf("invalid on")
	}
	if flagLeftPrefix != "" {
		flagLeftPrefix = strings.TrimSuffix(flagLeftPrefix, "_") + "_"
	}
	if flagRightPrefix != "" {
		flagRightPrefix = strings.TrimSuffix(flagRightPrefix, "_") + "_"
	}

	var output *os.File
	if flagOutput == "-" {
		output = os.Stdout
	} else {
		if exists, err := util.FileExists(flagOutput); err != nil || (exists && !flagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", flagOutput, err)
		}
		output, err = os.Create(flagOutput)
		if err != nil {
			return nil
		}
		defer output.Close()
	}
	err = joinCsvFiles(argLeftCsv, argRightCsv, output, leftOn, rightOn, flagLeftPrefix, flagRightPrefix, flagFullJoin)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	// Add the join command to the root CSV command
	joinCmd.Flags().BoolVarP(&flagFullJoin, "full-join", "", false, `Perform a full outer join instead of a left join.`)
	joinCmd.Flags().StringVarP(&flagOutput, "output", "o", "-", `Output file path. Use "-" for stdout.`)
	joinCmd.Flags().StringVarP(&flagOn, "on", "", "", `join on field, can be "left_field:right_field" format`)
	joinCmd.Flags().StringVarP(&flagLeftPrefix, "left-prefix", "", "", `Prefix columns of left csv with this string + "_"`)
	joinCmd.Flags().StringVarP(&flagRightPrefix, "right-prefix", "", "", `Prefix columns of right csv with this string + "_"`)
	joinCmd.MarkFlagRequired("on")
	csv.CsvCmd.AddCommand(joinCmd)
}
