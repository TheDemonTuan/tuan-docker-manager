//go:build linux

package metrics

import (
	"syscall"
)

func getDiskUsage(path string) (total uint64, used uint64, free uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free = stat.Bavail * uint64(stat.Bsize)
	used = (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	return total, used, free, nil
}
