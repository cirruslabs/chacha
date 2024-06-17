package run

import (
	"bytes"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache/disk"
	"github.com/cirruslabs/chacha/internal/cache/s3"
	"github.com/cirruslabs/chacha/internal/config"
	serverpkg "github.com/cirruslabs/chacha/internal/server"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"net/url"
	"os"
)

var addr string
var configPath string

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the Chacha server",
		RunE:  run,
	}

	cmd.Flags().StringVarP(&addr, "listen", "l", ":8080",
		"address to listen on")
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

	config, err := config.Parse(bytes.NewReader(configBytes))
	if err != nil {
		return err
	}

	maxBytes, err := humanize.ParseBytes(config.Disk.Limit)
	if err != nil {
		return err
	}

	localCache, err := disk.New(config.Disk.Dir, maxBytes)
	if err != nil {
		return err
	}

	remoteCache, err := s3.New(cmd.Context(), config.S3.Bucket)
	if err != nil {
		return err
	}

	server, err := serverpkg.New(addr, (*url.URL)(&config.BaseURL), config.OIDCProviders, localCache, remoteCache)
	if err != nil {
		return err
	}

	return server.Run(cmd.Context())
}
