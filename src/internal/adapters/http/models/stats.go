package models

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"xashloger/internal/core/domain"
)

const maxCPUPoints = 24 * 60

var (
	lastCPU     domain.CPUStat
	lastCPULock sync.Mutex

	cpuHistoryByDay = make(map[string][]domain.CPUPoint)
	cpuLock         sync.Mutex
)

func StartCPUCollector() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		day := now.Format("2006-01-02")

		cur, err := getCPUStat()
		if err != nil {
			continue
		}

		lastCPULock.Lock()

		var cpu float64

		if lastCPU.User == 0 {
			lastCPU = cur
			lastCPULock.Unlock()
			continue
		}

		if lastCPU.User != 0 {
			cpu = cpuUsage(lastCPU, cur)
		}
		lastCPU = cur
		lastCPULock.Unlock()

		point := domain.CPUPoint{
			Time:  now.Format("2006-01-02 15:04"),
			Value: cpu,
		}

		cpuLock.Lock()

		cpuHistoryByDay[day] = append(cpuHistoryByDay[day], point)
		if len(cpuHistoryByDay[day]) > maxCPUPoints {
			cpuHistoryByDay[day] = cpuHistoryByDay[day][len(cpuHistoryByDay[day])-maxCPUPoints:]
		}

		// ограничение: 7 дней
		if len(cpuHistoryByDay) > 7 {
			cutoff := now.AddDate(0, 0, -7).Format("2006-01-02")
			delete(cpuHistoryByDay, cutoff)
		}

		cpuLock.Unlock()
	}
}

func GetCPUHistory(period string) []domain.CPUPoint {
	cpuLock.Lock()
	defer cpuLock.Unlock()

	today := time.Now()
	result := []domain.CPUPoint{}

	switch period {
	case "today":
		result = cpuHistoryByDay[today.Format("2006-01-02")]

	case "yesterday":
		result = cpuHistoryByDay[today.AddDate(0, 0, -1).Format("2006-01-02")]

	case "week":
		for i := 6; i >= 0; i-- {
			day := today.AddDate(0, 0, -i).Format("2006-01-02")
			result = append(result, cpuHistoryByDay[day]...)
		}
	}

	return result
}

func GetMemoryUsageMB() (used int64, total int64, err error) {
	get := func(name string) (int64, error) {
		out, err := exec.Command("sysctl", "-n", name).Output()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	}

	totalBytes, err := get("hw.physmem")
	if err != nil {
		return
	}

	pageSize, err := get("hw.pagesize")
	if err != nil {
		return
	}

	freePages, err := get("vm.stats.vm.v_free_count")
	if err != nil {
		return
	}

	inactivePages, err := get("vm.stats.vm.v_inactive_count")
	if err != nil {
		return
	}

	cachePages, err := get("vm.stats.vm.v_cache_count")
	if err != nil {
		return
	}

	freeBytes :=
		(freePages + inactivePages + cachePages) * pageSize

	total = totalBytes / 1024 / 1024
	used = (totalBytes - freeBytes) / 1024 / 1024

	return
}

func cpuUsage(prev, cur domain.CPUStat) float64 {
	idle := cur.Idle - prev.Idle
	total :=
		(cur.User - prev.User) +
			(cur.Nice - prev.Nice) +
			(cur.System - prev.System) +
			(cur.Interrupt - prev.Interrupt) +
			idle

	if total == 0 {
		return 0
	}

	return 100 * float64(total-idle) / float64(total)
}

func getCPUStat() (domain.CPUStat, error) {
	out, err := exec.Command("sysctl", "-n", "kern.cp_time").Output()
	if err != nil {
		return domain.CPUStat{}, err
	}

	fields := strings.Fields(string(out))
	if len(fields) < 5 {
		return domain.CPUStat{}, fmt.Errorf("unexpected cp_time format")
	}

	parse := func(s string) uint64 {
		v, _ := strconv.ParseUint(s, 10, 64)
		return v
	}

	return domain.CPUStat{
		User:      parse(fields[0]),
		Nice:      parse(fields[1]),
		System:    parse(fields[2]),
		Interrupt: parse(fields[3]),
		Idle:      parse(fields[4]),
	}, nil
}
