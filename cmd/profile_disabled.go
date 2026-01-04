//go:build !profile

package cmd

import "flag"

type profiler struct{}

type profileFlags struct{}

func registerProfileFlags(_ *flag.FlagSet) profileFlags {
	return profileFlags{}
}

func startProfilerFromFlags(_ profileFlags) (*profiler, error) {
	return nil, nil
}

func (*profiler) stop() error {
	return nil
}
