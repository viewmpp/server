package vcs

import (
	"fmt"
	"runtime/debug"
	"time"
)

func Version() string {
	var revision string
	var modified bool
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					modified = true
				}
			}
		}
		if revision == "" {
			return "dev mode"
		}
		if modified {
			return fmt.Sprintf("%s-dirty", revision)
		}
	}
	return revision
}

func Time() (time.Time, bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return time.Time{}, false
	}

	for _, s := range bi.Settings {
		if s.Key != "vcs.time" {
			continue
		}

		at, err := time.Parse(time.RFC3339, s.Value)
		if err != nil {
			return time.Time{}, false
		}

		return at, true
	}

	return time.Time{}, false
}
