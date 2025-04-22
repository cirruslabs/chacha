package localnetworkhelper

import "errors"

//nolint:staticcheck // it's OK to capitalize "Local Network" in this error
var ErrUnsupportedPlatform = errors.New("Local Network permission helper is not supported on this platform")
