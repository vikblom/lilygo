package debug

import (
	"runtime/debug"
)

func BuildInfo() string {
	sha := "(dev)"
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, v := range info.Settings {
			if v.Key == "vcs.revision" {
				sha = v.Value
			}
		}
	}
	return sha
}
