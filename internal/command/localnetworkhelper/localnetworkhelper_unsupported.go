//go:build !darwin

package localnetworkhelper

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return nil
}
