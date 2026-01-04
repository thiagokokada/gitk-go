package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	t.Run("prefers module version", func(t *testing.T) {
		original := version
		version = "nix-abc123"
		t.Cleanup(func() { version = original })

		info := &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
		}

		if got := versionFromBuildInfo(info); got != "v1.2.3" {
			t.Fatalf("versionFromBuildInfo() = %q, want %q", got, "v1.2.3")
		}
	})

	t.Run("uses nix version when module is devel", func(t *testing.T) {
		original := version
		version = "nix-abc123"
		t.Cleanup(func() { version = original })

		info := &debug.BuildInfo{
			Main: debug.Module{Version: "(devel)"},
		}

		if got := versionFromBuildInfo(info); got != "nix-abc123" {
			t.Fatalf("versionFromBuildInfo() = %q, want %q", got, "nix-abc123")
		}
	})

	t.Run("falls back to dev", func(t *testing.T) {
		original := version
		version = ""
		t.Cleanup(func() { version = original })

		if got := versionFromBuildInfo(nil); got != "dev" {
			t.Fatalf("versionFromBuildInfo() = %q, want %q", got, "dev")
		}
	})

	t.Run("uses nix version without build info", func(t *testing.T) {
		original := version
		version = "nix-abc123"
		t.Cleanup(func() { version = original })

		if got := versionFromBuildInfo(nil); got != "nix-abc123" {
			t.Fatalf("versionFromBuildInfo() = %q, want %q", got, "nix-abc123")
		}
	})
}
