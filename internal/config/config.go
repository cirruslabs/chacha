package config

import (
	"gopkg.in/yaml.v3"
	"io"
)

type Config struct {
	OIDCProviders []OIDCProvider `yaml:"oidc-providers"`
	Disk          Disk           `yaml:"disk"`
	S3            S3             `yaml:"s3"`
}

type OIDCProvider struct {
	URL           string   `yaml:"url"`
	CacheKeyExprs []string `yaml:"cache_key_exprs"`
}

type Disk struct {
	Dir   string `yaml:"dir"`
	Limit string `yaml:"limit"`
}

type S3 struct {
	Bucket string `yaml:"bucket"`
}

func Parse(r io.Reader) (*Config, error) {
	var config Config

	decoder := yaml.NewDecoder(r)

	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
