package percentencoding_test

import (
	"github.com/cirruslabs/chacha/internal/cache/disk/percentencoding"
	"github.com/stretchr/testify/require"
	"testing"
	"testing/quick"
)

func TestQuickCheck(t *testing.T) {
	f := func(original string) bool {
		transformed, err := transform(original)
		if err != nil {
			panic(err)
		}

		return original == transformed
	}

	require.NoError(t, quick.Check(f, &quick.Config{
		MaxCount: 100_000,
	}))
}

func transform(s string) (string, error) {
	encoded := percentencoding.Encode(s)

	return percentencoding.Decode(encoded)
}
