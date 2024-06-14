package s3

import (
	"context"
	s3pkg "github.com/aws/aws-sdk-go-v2/service/s3"
	transport "github.com/aws/smithy-go/endpoints"
	"net/url"
)

type s3EndpointResolver struct {
	url *url.URL
}

func (e *s3EndpointResolver) ResolveEndpoint(
	_ context.Context,
	_ s3pkg.EndpointParameters,
) (transport.Endpoint, error) {
	return transport.Endpoint{
		URI: *e.url,
	}, nil
}
