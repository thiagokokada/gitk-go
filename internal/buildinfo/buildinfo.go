package buildinfo

import (
	"fmt"
	"runtime/debug"
)

// Version returns the module version or "dev" when unset.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return "dev"
	}
	version := info.Main.Version
	if version == "" || version == "(devel)" {
		return "dev"
	}
	return version
}

// Tags returns the GOFLAGS build tags recorded at compile time.
func Tags() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info == nil {
		return ""
	}
	for _, setting := range info.Settings {
		if setting.Key == "-tags" {
			return setting.Value
		}
	}
	return ""
}

// VersionWithTags returns the version string and tags if present.
func VersionWithTags() string {
	version := Version()
	tags := Tags()
	if tags == "" {
		return version
	}
	return fmt.Sprintf("%s (tags: %s)", version, tags)
}
