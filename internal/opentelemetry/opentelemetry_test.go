package opentelemetry_test

import (
	"context"
	"github.com/cirruslabs/chacha/internal/opentelemetry"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInit(t *testing.T) {
	opentelemetryDeinit, err := opentelemetry.Init(context.Background())
	require.NoError(t, err)
	opentelemetryDeinit()
}
