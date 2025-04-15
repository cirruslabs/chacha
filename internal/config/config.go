package config

import (
	"gopkg.in/yaml.v3"
	"io"
)

type Config struct {
	Addr           string          `yaml:"addr"`
	Disk           *Disk           `yaml:"disk"`
	TLSInterceptor *TLSInterceptor `yaml:"tls-interceptor"`
	Rules          []Rule          `yaml:"rules"`
	Cluster        *Cluster        `yaml:"cluster"`
}

type Disk struct {
	Dir   string `yaml:"dir"`
	Limit string `yaml:"limit"`
}

type TLSInterceptor struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

type Rule struct {
	Pattern                   string   `yaml:"pattern"`
	IgnoreAuthorizationHeader bool     `yaml:"ignore-authorization-header"`
	IgnoreParameters          []string `yaml:"ignore-parameters"`
	DirectConnect             bool     `yaml:"direct-connect"`
	DirectConnectHeader       bool     `yaml:"direct-connect-header"`
}

type Cluster struct {
	Secret string `yaml:"secret"`
	Nodes  []Node `yaml:"nodes"`
}

type Node struct {
	Addr string `yaml:"addr"`
}

func Parse(r io.Reader) (*Config, error) {
	var config Config

	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
