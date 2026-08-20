// Package metrics reads live host statistics (CPU, memory, load, uptime,
// disk) for the admin metrics dashboard.
//
// The deployment runs the api container ON the host it reports on, and a
// Linux container shares the host kernel, so /proc/stat, /proc/meminfo,
// /proc/loadavg and /proc/uptime expose whole-box values with no extra
// tooling or cloud credentials. Disk usage is read via statfs on the host
// filesystem mount (compose mounts the host root read-only at /host); when
// that mount is absent (e.g. tests) it falls back to the container root.
package metrics

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// HostDiskPath is the path of the host filesystem bind-mounted read-only into
// the container; disk stats fall back to "/" when it does not exist.
const HostDiskPath = "/host"

// HostStats is a single point-in-time snapshot of the machine the api runs on.
type HostStats struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemTotalBytes  uint64  `json:"mem_total"`
	MemUsedBytes   uint64  `json:"mem_used"`
	MemPercent     float64 `json:"mem_percent"`
	Load1          float64 `json:"load1"`
	Load5          float64 `json:"load5"`
	Load15         float64 `json:"load15"`
	UptimeSeconds  float64 `json:"uptime_seconds"`
	DiskTotalBytes uint64  `json:"disk_total"`
	DiskUsedBytes  uint64  `json:"disk_used"`
	DiskPercent    float64 `json:"disk_percent"`
}

// ReadHostStats gathers a fresh snapshot. CPU percent is a busy/total ratio
// over a ~200ms sample window, hence the small delay.
func ReadHostStats() (*HostStats, error) {
	memTotal, memUsed, err := readMemInfo()
	if err != nil {
		return nil, err
	}
	load1, load5, load15, err := readLoadAvg()
	if err != nil {
		return nil, err
	}
	uptime, err := readUptime()
	if err != nil {
		return nil, err
	}
	diskTotal, diskUsed, err := readDiskUsage()
	if err != nil {
		return nil, err
	}

	stats := &HostStats{
		MemTotalBytes:  memTotal,
		MemUsedBytes:   memUsed,
		MemPercent:     percent(memUsed, memTotal),
		Load1:          load1,
		Load5:          load5,
		Load15:         load15,
		UptimeSeconds:  uptime,
		DiskTotalBytes: diskTotal,
		DiskUsedBytes:  diskUsed,
		DiskPercent:    percent(diskUsed, diskTotal),
	}

	cpuPercent, err := readCPUPercent()
	if err != nil {
		return nil, err
	}
	stats.CPUPercent = cpuPercent
	return stats, nil
}

// percent returns used/total*100, or 0 when total is zero.
func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// readCPUPercent computes CPU busy percentage from two /proc/stat samples.
func readCPUPercent() (float64, error) {
	first, err := sampleCPUTime()
	if err != nil {
		return 0, err
	}
	time.Sleep(200 * time.Millisecond)
	second, err := sampleCPUTime()
	if err != nil {
		return 0, err
	}
	total := float64(second.total - first.total)
	if total <= 0 {
		return 0, nil
	}
	busy := float64(second.busy - first.busy)
	return busy / total * 100, nil
}

type cpuSample struct {
	total uint64
	busy  uint64
}

// sampleCPUTime parses the first line of /proc/stat, summing all JIFFY
// counters. busy excludes idle and iowait.
func sampleCPUTime() (cpuSample, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return cpuSample{}, errors.New("metrics: empty /proc/stat")
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, fmt.Errorf("metrics: unexpected /proc/stat line: %q", scanner.Text())
	}
	values := make([]uint64, len(fields)-1)
	for i, f := range fields[1:] {
		values[i], err = strconv.ParseUint(f, 10, 64)
		if err != nil {
			return cpuSample{}, fmt.Errorf("metrics: parse cpu field %q: %w", f, err)
		}
	}
	var total uint64
	for _, v := range values {
		total += v
	}
	// values: user nice system idle iowait irq softirq steal
	idle := values[3] + values[4]
	return cpuSample{total: total, busy: total - idle}, nil
}

// readMemInfo parses /proc/meminfo. "Used" = total - free - buffers - cached.
func readMemInfo() (total, used uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	var free, buffers, cached uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = value
		case "MemFree":
			free = value
		case "Buffers":
			buffers = value
		case "Cached":
			cached = value
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}
	if total == 0 {
		return 0, 0, errors.New("metrics: MemTotal not found in /proc/meminfo")
	}
	used = total - free - buffers - cached
	return total, used, nil
}

// readLoadAvg parses the three load-average figures from /proc/loadavg.
func readLoadAvg() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("metrics: unexpected /proc/loadavg: %q", string(data))
	}
	values := make([]float64, 3)
	for i := 0; i < 3; i++ {
		values[i], err = strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("metrics: parse loadavg %q: %w", fields[i], err)
		}
	}
	return values[0], values[1], values[2], nil
}

// readUptime parses system uptime seconds from /proc/uptime.
func readUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("metrics: unexpected /proc/uptime: %q", string(data))
	}
	return strconv.ParseFloat(fields[0], 64)
}

// readDiskUsage statfs's the host filesystem mount (or the container root in
// its absence) and reports capacity minus free space.
func readDiskUsage() (total, used uint64, err error) {
	path := HostDiskPath
	if _, statErr := os.Stat(path); statErr != nil {
		path = "/"
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total < free {
		return total, 0, nil
	}
	return total, total - free, nil
}
