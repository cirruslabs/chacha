//go:build !unix

package localnetworkhelper

func Serve(_ int) error {
	return ErrUnsupportedPlatform
}
