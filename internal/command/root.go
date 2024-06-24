package command

import (
	"github.com/cirruslabs/chacha/internal/command/run"
	"github.com/cirruslabs/chacha/internal/logginglevel"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

var debug bool

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "chacha",
		Short:         "Caching proxy server for Cirrus Runners infrastructure",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if debug {
				logginglevel.Level.SetLevel(zapcore.DebugLevel)
			}

			return nil
		},
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	cmd.AddCommand(
		run.NewCommand(),
	)

	return cmd
}
