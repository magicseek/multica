package execenv

import "errors"

var errCloneUnsupported = errors.New("copy-on-write clone unsupported")
