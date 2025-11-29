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
	Use:   "join --on <on_field> <left.csv> <right.csv>",
	Short: "merge (left join) two csv",
	Long: `merge (left join) two csv.

Read two csv files, "join" them and output a new csv. Output merged csv to stdout by default.

- It's similar to SQL left join semantic, but for each left csv row,
  at most one (1) matched right csv row is allowed; if multiple right row match, use the first one.
- The result csv contains all rows & columns of left join, along with columns of right csv
  when leftCsv.leftOn field == rightCsv.rightOn field (must not be empty string).
- If a same column name exists in both left and right csv, use the left version as output csv column value.
- If leftPrefix / rightPrefix is not empty, Prefix columns of left / right csv with this string in output csv.
- If allJoin is true, do a "full join" instead of "left join": include all rows of both csv in output.
- If all right csv column names are "masked" by left csv, it returns an error.
- If noHeader is true, input files are treated as having no header row; columns are named c1, c2, c3...`,
	Args: cobra.ExactArgs(2),
	RunE: join,
}

var (
	flagOn          string // join on field, can be "left_field:right_field" format
	flagLeftPrefix  string // Prefix columns of left csv with this string + "_"
	flagRightPrefix string // Prefix columns of right csv with this string + "_"
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
	if csv.FlagOutput == "-" {
		output = os.Stdout
	} else {
		if exists, err := util.FileExists(csv.FlagOutput); err != nil || (exists && !csv.FlagForce) {
			return fmt.Errorf("output file %q exists or can't access, err=%w", csv.FlagOutput, err)
		}
		output, err = os.Create(csv.FlagOutput)
		if err != nil {
			return nil
		}
		defer output.Close()
	}
	err = joinCsvFiles(argLeftCsv, argRightCsv, output, leftOn, rightOn,
		flagLeftPrefix, flagRightPrefix, flagFullJoin, csv.FlagNoHeader)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	// Add the join command to the root CSV command
	joinCmd.Flags().BoolVarP(&flagFullJoin, "full-join", "", false, `Perform a full outer join instead of a left join.`)
	joinCmd.Flags().StringVarP(&flagOn, "on", "", "", `join on field, can be "left_field:right_field" format`)
	joinCmd.Flags().StringVarP(&flagLeftPrefix, "left-prefix", "", "",
		`Prefix columns of left csv with this string + "_"`)
	joinCmd.Flags().StringVarP(&flagRightPrefix, "right-prefix", "", "",
		`Prefix columns of right csv with this string + "_"`)
	joinCmd.MarkFlagRequired("on")
	csv.CsvCmd.AddCommand(joinCmd)
}
