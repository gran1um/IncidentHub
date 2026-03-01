package api

import (
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	memstat "github.com/shirou/gopsutil/v3/mem"
)

type hostResourceStats struct {
	CPUPercent        float64 `json:"cpuPercent"`
	CPUCoresUsed      float64 `json:"cpuCoresUsed"`
	CPUCoresTotal     int     `json:"cpuCoresTotal"`
	MemoryUsedBytes   uint64  `json:"memoryUsedBytes"`
	MemoryTotalBytes  uint64  `json:"memoryTotalBytes"`
	MemoryUsedMB      float64 `json:"memoryUsedMB"`
	MemoryTotalMB     float64 `json:"memoryTotalMB"`
	MemoryUsedGB      float64 `json:"memoryUsedGB"`
	MemoryTotalGB     float64 `json:"memoryTotalGB"`
	MemoryUsedPercent float64 `json:"memoryUsedPercent"`
	DiskUsedBytes     uint64  `json:"diskUsedBytes"`
	DiskTotalBytes    uint64  `json:"diskTotalBytes"`
	DiskUsedGB        float64 `json:"diskUsedGB"`
	DiskTotalGB       float64 `json:"diskTotalGB"`
	DiskUsedPercent   float64 `json:"diskUsedPercent"`
	Goroutines        int     `json:"goroutines"`
}

func (h *Handler) collectHostResources() hostResourceStats {
	stats := hostResourceStats{}
	stats.Goroutines = runtime.NumGoroutine()

	stats.CPUCoresTotal = runtime.NumCPU()
	if detectedCores, err := cpu.Counts(true); err == nil && detectedCores > 0 {
		stats.CPUCoresTotal = detectedCores
	}
	if usageSamples, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(usageSamples) > 0 {
		stats.CPUPercent = roundTo(usageSamples[0])
	}
	if stats.CPUCoresTotal > 0 && stats.CPUPercent > 0 {
		stats.CPUCoresUsed = roundTo(float64(stats.CPUCoresTotal) * stats.CPUPercent / 100)
	}

	if virtualMemory, err := memstat.VirtualMemory(); err == nil {
		stats.MemoryUsedBytes = virtualMemory.Used
		stats.MemoryTotalBytes = virtualMemory.Total
		stats.MemoryUsedMB = roundTo(float64(virtualMemory.Used) / (1024 * 1024))
		stats.MemoryTotalMB = roundTo(float64(virtualMemory.Total) / (1024 * 1024))
		stats.MemoryUsedGB = roundTo(float64(virtualMemory.Used) / (1024 * 1024 * 1024))
		stats.MemoryTotalGB = roundTo(float64(virtualMemory.Total) / (1024 * 1024 * 1024))
		stats.MemoryUsedPercent = roundTo(virtualMemory.UsedPercent)
	} else {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		stats.MemoryUsedBytes = mem.Sys
		stats.MemoryUsedMB = roundTo(float64(mem.Sys) / (1024 * 1024))
		stats.MemoryUsedGB = roundTo(float64(mem.Sys) / (1024 * 1024 * 1024))
	}

	if usage, err := disk.Usage(hostDiskUsagePath()); err == nil {
		stats.DiskTotalBytes = usage.Total
		stats.DiskUsedBytes = usage.Used
		stats.DiskTotalGB = roundTo(float64(usage.Total) / (1024 * 1024 * 1024))
		stats.DiskUsedGB = roundTo(float64(usage.Used) / (1024 * 1024 * 1024))
		stats.DiskUsedPercent = roundTo(usage.UsedPercent)
	}

	return stats
}

func hostDiskUsagePath() string {
	if runtime.GOOS == "windows" {
		drive := strings.TrimSpace(os.Getenv("SystemDrive"))
		if drive == "" {
			drive = "C:"
		}
		if strings.HasSuffix(drive, "\\") {
			return drive
		}
		return drive + "\\"
	}
	return "/"
}

func roundTo(value float64) float64 {
	const pow = 100.0
	return math.Round(value*pow) / pow
}
