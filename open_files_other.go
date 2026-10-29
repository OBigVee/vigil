//go:build !darwin

package main

import (
	"context"

	"github.com/shirou/gopsutil/v4/process"
)

func processOpenFiles(ctx context.Context, p *process.Process) ([]process.OpenFilesStat, error) {
	return p.OpenFilesWithContext(ctx)
}
