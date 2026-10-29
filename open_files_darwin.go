package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// lsof is supplied by macOS. NUL-delimited fields preserve whitespace in paths.
// Only target PID metadata is read; arguments, environment and file contents are excluded.
func processOpenFiles(ctx context.Context, p *process.Process) ([]process.OpenFilesStat, error) {
	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(queryCtx, "/usr/sbin/lsof", "-nP", "-a", "-p", strconv.Itoa(int(p.Pid)), "-F0ftn")
	output := &limitedOutput{remaining: 8 << 20}
	cmd.Stdout = output
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("macOS open-file metadata unavailable")
	}
	if output.truncated {
		return nil, fmt.Errorf("macOS open-file metadata exceeded the collection limit")
	}
	return parseLsofFiles([]byte(output.String())), nil
}
