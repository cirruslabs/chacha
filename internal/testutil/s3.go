package testutil

import (
	"context"
	"fmt"
	"github.com/cirruslabs/chacha/internal/cache/s3"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
)

func S3(t *testing.T) *s3.Config {
	t.Helper()

	ctx := context.Background()

	localstackContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack",
			WaitingFor:   wait.ForHTTP("/_localstack/health").WithPort("4566/tcp"),
			ExposedPorts: []string{"4566/tcp"},
		},
		Started: true,
	})
	require.NoError(t, err)

	exposedPort, err := nat.NewPort("tcp", "4566")
	require.NoError(t, err)

	mappedPort, err := localstackContainer.MappedPort(ctx, exposedPort)
	require.NoError(t, err)

	return &s3.Config{
		Endpoint:        fmt.Sprintf("http://test.s3.localhost.localstack.cloud:%d/", mappedPort.Int()),
		Region:          "us-east-1",
		Bucket:          "test",
		AccessKeyID:     "key-id",
		AccessKeySecret: "key-secret",
	}
}
