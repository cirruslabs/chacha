// Package percentencoding implements a much stricter form of URL encoding[1]
// suitable for use for OS filesystem operations without the need of things like
// SecureJoin[2].
//
// [1]: https://en.wikipedia.org/wiki/Percent-encoding
// [2]: https://github.com/cyphar/filepath-securejoin
package percentencoding

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrIncompleteInput = errors.New("incomplete input")

func Encode(s string) string {
	var result strings.Builder

	for _, c := range []byte(s) {
		switch {
		case c >= '0' && c <= '9':
			fallthrough
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
			fallthrough
		case c == '-' || c == '_':
			result.WriteByte(c)
		default:
			result.WriteString(fmt.Sprintf("%%%02x", c))
		}
	}

	return result.String()
}

func Decode(s string) (string, error) {
	var result strings.Builder

	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			if (i + 2) > len(s) {
				return "", ErrIncompleteInput
			}

			value, err := strconv.ParseUint(s[i+1:i+3], 16, 8)
			if err != nil {
				return "", err
			}

			i += 2

			result.WriteByte(byte(value))
		} else {
			result.WriteByte(s[i])
		}
	}

	return result.String(), nil
}
