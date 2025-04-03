package command

import (
	"github.com/cirruslabs/chacha/internal/command/localnetworkhelper"
	"github.com/cirruslabs/chacha/internal/command/run"
	"github.com/cirruslabs/chacha/internal/logginglevel"
	"github.com/cirruslabs/chacha/internal/version"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

var debug bool

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "chacha",
		Short:         "Caching proxy server for Cirrus Runners infrastructure",
		Version:       version.FullVersion,
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

	commands := []*cobra.Command{
		run.NewCommand(),
	}

	if localNetworkHelperCommand := localnetworkhelper.NewCommand(); localNetworkHelperCommand != nil {
		commands = append(commands, localNetworkHelperCommand)
	}

	cmd.AddCommand(commands...)

	return cmd
}
