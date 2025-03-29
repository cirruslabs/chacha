package kv

import "net/http"

type Option func(kv *KV)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(kv *KV) {
		kv.httpClient = httpClient
	}
}
