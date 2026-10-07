package sysmon

import (
	"strconv"
	"strings"
)

// parseMeminfo reads /proc/meminfo into bytes, by field name.
func parseMeminfo(data string) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(data, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			continue
		}
		if len(f) > 1 && strings.EqualFold(f[1], "kB") {
			n *= 1024
		}
		out[k] = n
	}
	return out
}

func memorySample(m map[string]uint64) MemorySample {
	total, avail := m["MemTotal"], m["MemAvailable"]
	if avail == 0 {
		// Old kernels have no MemAvailable.
		avail = m["MemFree"] + m["Cached"] + m["Buffers"]
	}
	if avail > total {
		avail = total
	}
	s := MemorySample{
		Total: total, Available: avail, Used: total - avail,
		Free: m["MemFree"], Cached: m["Cached"], Buffers: m["Buffers"],
		SwapTotal: m["SwapTotal"],
	}
	if m["SwapFree"] <= m["SwapTotal"] {
		s.SwapUsed = m["SwapTotal"] - m["SwapFree"]
	}
	return s
}
