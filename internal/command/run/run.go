package run

import (
	"bytes"
	"fmt"
	diskpkg "github.com/cirruslabs/chacha/internal/cache/disk"
	configpkg "github.com/cirruslabs/chacha/internal/config"
	serverpkg "github.com/cirruslabs/chacha/internal/server"
	"github.com/cirruslabs/chacha/internal/server/cluster"
	"github.com/cirruslabs/chacha/internal/server/rule"
	"github.com/cirruslabs/chacha/internal/server/tlsinterceptor"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
)

var configPath string

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the Chacha server",
		RunE:  run,
	}

	cmd.Flags().StringVarP(&configPath, "file", "f", "",
		"configuration file path (e.g. /etc/chacha.yml)")

	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	if configPath == "" {
		return fmt.Errorf("configuration file path (-f or --file) needs to be specified")
	}

	// Parse the configuration file
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file at path %s: %w", configPath, err)
	}

	config, err := configpkg.Parse(bytes.NewReader(configBytes))
	if err != nil {
		return fmt.Errorf("failed to parse configuration file at path %s: %w", configPath, err)
	}

	opts := []serverpkg.Option{
		serverpkg.WithLogger(zap.S()),
	}

	if config.Disk != nil {
		limitBytes, err := humanize.ParseBytes(config.Disk.Limit)
		if err != nil {
			return fmt.Errorf("failed to parse disk limit value %q: %w", config.Disk.Limit, err)
		}

		disk, err := diskpkg.New(config.Disk.Dir, limitBytes)
		if err != nil {
			return err
		}

		opts = append(opts, serverpkg.WithDisk(disk))
	}

	if config.TLSInterceptor != nil {
		tlsInterceptor, err := tlsinterceptor.NewFromFiles(config.TLSInterceptor.Cert, config.TLSInterceptor.Key)
		if err != nil {
			return err
		}

		opts = append(opts, serverpkg.WithTLSInterceptor(tlsInterceptor))
	}

	if len(config.Rules) != 0 {
		var rules rule.Rules

		for _, configMatch := range config.Rules {
			rule, err := rule.New(configMatch.Pattern, configMatch.IgnoreAuthorizationHeader,
				configMatch.IgnoreParameters)
			if err != nil {
				return err
			}

			rules = append(rules, rule)
		}

		opts = append(opts, serverpkg.WithRules(rules))
	}

	if config.Cluster != nil {
		opts = append(opts, serverpkg.WithCluster(cluster.New(config.Cluster.Secret,
			config.Addr, config.Cluster.Nodes)))
	}

	server, err := serverpkg.New(config.Addr, opts...)
	if err != nil {
		return err
	}

	return server.Run(cmd.Context())
}
