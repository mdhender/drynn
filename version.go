// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package drynn

import (
	"github.com/maloquacious/semver"
)

var (
	version = semver.Version{
		Major: 0,
		Minor: 1,
		Patch: 0,
		Build: semver.Commit(),
	}
)

func Version() semver.Version {
	return version
}
