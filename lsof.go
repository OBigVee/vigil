package main

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func parseLsofFiles(data []byte) []process.OpenFilesStat {
	files := []process.OpenFilesStat{}
	fd, kind, path := "", "", ""
	flush := func() {
		number, err := strconv.ParseUint(fd, 10, 64)
		if err == nil && (kind == "REG" || kind == "DIR") && strings.HasPrefix(path, "/") {
			files = append(files, process.OpenFilesStat{Path: path, Fd: number})
		}
	}
	for _, field := range bytes.Split(data, []byte{0}) {
		field = bytes.TrimLeft(field, "\n")
		if len(field) == 0 {
			continue
		}
		switch field[0] {
		case 'p':
			flush()
			fd, kind, path = "", "", ""
		case 'f':
			flush()
			fd, kind, path = string(field[1:]), "", ""
		case 't':
			kind = string(field[1:])
		case 'n':
			path = string(field[1:])
		}
	}
	flush()
	return files
}
