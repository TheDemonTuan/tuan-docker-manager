//go:build !linux

package metrics

func getDiskUsage(path string) (total uint64, used uint64, free uint64, err error) {
	return 200 * 1024 * 1024 * 1024, 42 * 1024 * 1024 * 1024, 158 * 1024 * 1024 * 1024, nil
}
