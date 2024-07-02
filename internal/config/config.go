package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/url"
)

type BaseURL url.URL

func (baseURL *BaseURL) UnmarshalYAML(value *yaml.Node) error {
	if value.Value == "" {
		return fmt.Errorf("base URL cannot be empty")
	}

	parsedURL, err := url.Parse(value.Value)
	if err != nil {
		return err
	}

	*baseURL = BaseURL(*parsedURL)

	return nil
}

type Config struct {
	BaseURL       *BaseURL       `yaml:"base_url"`
	OIDCProviders []OIDCProvider `yaml:"oidc_providers"`
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
