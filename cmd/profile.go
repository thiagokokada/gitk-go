//go:build profile

package cmd

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

type profiler struct {
	cpuFile *os.File
	memPath string
}

func startProfiler(cpuPath, memPath string) (*profiler, error) {
	if cpuPath == "" && memPath == "" {
		return nil, nil
	}
	p := &profiler{memPath: memPath}
	if cpuPath != "" {
		f, err := os.Create(cpuPath)
		if err != nil {
			return nil, fmt.Errorf("create cpu profile: %w", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("start cpu profile: %w", err)
		}
		p.cpuFile = f
	}
	return p, nil
}

func (p *profiler) stop() error {
	if p == nil {
		return nil
	}
	var errs []error
	if p.cpuFile != nil {
		pprof.StopCPUProfile()
		if err := p.cpuFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close cpu profile: %w", err))
		}
		p.cpuFile = nil
	}
	if p.memPath != "" {
		f, err := os.Create(p.memPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("create mem profile: %w", err))
		} else {
			runtime.GC()
			if err := pprof.WriteHeapProfile(f); err != nil {
				errs = append(errs, fmt.Errorf("write mem profile: %w", err))
			}
			if err := f.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close mem profile: %w", err))
			}
		}
	}
	return errors.Join(errs...)
}
