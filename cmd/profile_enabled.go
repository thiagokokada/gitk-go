//go:build profile

package cmd

import "flag"

type profileFlags struct {
	cpu *string
	mem *string
}

func registerProfileFlags(fs *flag.FlagSet) profileFlags {
	return profileFlags{
		cpu: fs.String("cpuprofile", "", "write CPU profile to file"),
		mem: fs.String("memprofile", "", "write heap profile to file on exit"),
	}
}

func startProfilerFromFlags(flags profileFlags) (*profiler, error) {
	return startProfiler(*flags.cpu, *flags.mem)
}
