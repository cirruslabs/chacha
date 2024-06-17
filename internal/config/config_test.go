package config_test

import (
	"github.com/cirruslabs/chacha/internal/config"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	configFile, err := os.Open(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	actualConfig, err := config.Parse(configFile)
	require.NoError(t, err)
	require.Equal(t, &config.Config{
		BaseURL: config.BaseURL{
			Scheme: "https",
			Host:   "example.com",
		},
		OIDCProviders: []config.OIDCProvider{
			{
				URL: "https://token.actions.githubusercontent.com",
				CacheKeyExprs: []string{
					`"github/" + claims.repository + "/" + claims.ref`,
					`"github/" + claims.repository`,
				},
			},
			{
				URL: "https://gitlab.com",
				CacheKeyExprs: []string{
					`"gitlab/" + claims.project_path + "/" + claims.ref_path`,
					`"gitlab/" + claims.project_path`,
				},
			},
		},
		Disk: config.Disk{
			Dir:   "/cache",
			Limit: "50GB",
		},
		S3: config.S3{
			Bucket: "chacha",
		},
	}, actualConfig)
}
