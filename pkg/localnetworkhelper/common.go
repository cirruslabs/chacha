package localnetworkhelper

import "errors"

const CommandName = "localnetworkhelper"

//nolint:staticcheck // it's OK to capitalize "Local Network" in this error
var ErrUnsupportedPlatform = errors.New("Local Network permission helper is not supported on this platform")
