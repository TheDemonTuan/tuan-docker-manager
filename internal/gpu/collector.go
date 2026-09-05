package gpu

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"docker-panel/internal/models"
)

type Collector struct {
	hasNvidiaSmi bool
}

func NewCollector() *Collector {
	_, err := exec.LookPath("nvidia-smi")
	return &Collector{
		hasNvidiaSmi: err == nil,
	}
}

func (c *Collector) Collect(ctx context.Context) (*models.GPUMetrics, error) {
	if !c.hasNvidiaSmi {
		return &models.GPUMetrics{
			Available: false,
			GPUs:      make([]models.GPUInfo, 0),
		}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=index,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,power.limit,fan.speed,driver_version",
		"--format=csv,noheader,nounits")

	out, err := cmd.Output()
	if err != nil {
		return &models.GPUMetrics{
			Available: false,
			GPUs:      make([]models.GPUInfo, 0),
		}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	gpus := make([]models.GPUInfo, 0, len(lines))
	driverVer := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 9 {
			continue
		}

		idx, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		name := strings.TrimSpace(parts[1])
		util, _ := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		memUsedMB, _ := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64)
		memTotalMB, _ := strconv.ParseUint(strings.TrimSpace(parts[4]), 10, 64)
		temp, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
		powerDraw, _ := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
		powerLimit, _ := strconv.ParseFloat(strings.TrimSpace(parts[7]), 64)
		fanSpeed, _ := strconv.ParseFloat(strings.TrimSpace(parts[8]), 64)

		if len(parts) >= 10 {
			driverVer = strings.TrimSpace(parts[9])
		}

		gpus = append(gpus, models.GPUInfo{
			ID:          idx,
			Name:        name,
			Utilization: util,
			VRAMUsed:    memUsedMB * 1024 * 1024,
			VRAMTotal:   memTotalMB * 1024 * 1024,
			Temperature: temp,
			PowerDraw:   powerDraw,
			PowerLimit:  powerLimit,
			FanSpeed:    fanSpeed,
		})
	}

	return &models.GPUMetrics{
		Available:     len(gpus) > 0,
		DriverVersion: driverVer,
		GPUs:          gpus,
	}, nil
}
