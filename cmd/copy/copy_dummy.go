//go:build !windows

// https://github.com/golang-design/clipboard requires CGO or external dependencies on non-Windows platform,
// which is not what I want.

package copy

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func doCopy(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("%s is not supported", runtime.GOOS)
}
