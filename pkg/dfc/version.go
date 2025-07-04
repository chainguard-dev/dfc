/*
Copyright 2025 Chainguard, Inc.
SPDX-License-Identifier: Apache-2.0
*/

package dfc

import (
	"fmt"
	"runtime/debug"
	"sync"
)

var (
	once        sync.Once
	dfcVersion  = "dev"
	dfcRevision = ""
)

func Version() string {
	once.Do(func() {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}

		// Get the main module version
		if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			dfcVersion = bi.Main.Version
		}

		// Get the vcs revision from build settings
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.revision" {
				dfcRevision = setting.Value
				break
			}
		}
	})

	if dfcRevision != "" {
		return fmt.Sprintf("%s (%s)", dfcVersion, dfcRevision)
	}
	return dfcVersion
}
