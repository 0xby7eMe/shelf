package sysmon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cpuTimes struct{ busy, total uint64 }

// parseCPUStat reads /proc/stat: the line for all processors, then one per
// logical processor. Time spent waiting for input and output counts as idle.
func parseCPUStat(data string) (all cpuTimes, cores []cpuTimes) {
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		var v [8]uint64
		for i := 0; i < 8 && i+1 < len(f); i++ {
			v[i], _ = strconv.ParseUint(f[i+1], 10, 64)
		}
		// user nice system idle iowait irq softirq steal
		total := v[0] + v[1] + v[2] + v[3] + v[4] + v[5] + v[6] + v[7]
		t := cpuTimes{busy: total - v[3] - v[4], total: total}
		if f[0] == "cpu" {
			all = t
		} else {
			cores = append(cores, t)
		}
	}
	return all, cores
}

func usagePct(prev, cur cpuTimes) float64 {
	if cur.total <= prev.total {
		return 0
	}
	busy := float64(cur.busy-prev.busy) / float64(cur.total-prev.total) * 100
	return clamp(busy, 0, 100)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func ptr(v float64) *float64 { return &v }

// cpuFrequency averages the current clock of every logical processor, in MHz.
func (s *Sampler) cpuFrequency() float64 {
	dirs, _ := filepath.Glob(s.sys + "/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
	var sum float64
	n := 0
	for _, p := range dirs {
		if khz, ok := readFloat(p); ok {
			sum += khz / 1000
			n++
		}
	}
	if n > 0 {
		return sum / float64(n)
	}
	// Without cpufreq, /proc/cpuinfo has a rough figure.
	for _, line := range strings.Split(s.readProc("cpuinfo"), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "cpu MHz" {
			if mhz, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				sum += mhz
				n++
			}
		}
	}
	if n > 0 {
		return sum / float64(n)
	}
	return 0
}

func readFloat(path string) (float64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	return v, err == nil
}

func readString(path string) string {
	b, _ := os.ReadFile(path)
	return strings.TrimSpace(string(b))
}

// hwmons lists the sensor chips, with their names.
func (s *Sampler) hwmons() map[string]string {
	out := map[string]string{}
	dirs, _ := filepath.Glob(s.sys + "/class/hwmon/hwmon*")
	for _, d := range dirs {
		out[d] = readString(d + "/name")
	}
	return out
}

// cpuTemp is the processor's package temperature in degrees, if a sensor has it.
func (s *Sampler) cpuTemp() *float64 {
	for dir, name := range s.hwmons() {
		switch name {
		case "k10temp", "zenpower":
			// Tctl is temp1 on AMD; prefer Tdie when there is one.
			if v := labelledTemp(dir, "Tdie"); v != nil {
				return v
			}
			if v, ok := readFloat(dir + "/temp1_input"); ok {
				return ptr(v / 1000)
			}
		case "coretemp":
			if v := labelledTemp(dir, "Package id 0"); v != nil {
				return v
			}
			if v, ok := readFloat(dir + "/temp1_input"); ok {
				return ptr(v / 1000)
			}
		}
	}
	return nil
}

func labelledTemp(dir, label string) *float64 {
	labels, _ := filepath.Glob(dir + "/temp*_label")
	for _, l := range labels {
		if readString(l) == label {
			if v, ok := readFloat(strings.TrimSuffix(l, "_label") + "_input"); ok {
				return ptr(v / 1000)
			}
		}
	}
	return nil
}

func (s *Sampler) countProcesses() int {
	entries, _ := os.ReadDir(s.proc)
	n := 0
	for _, e := range entries {
		if isPID(e.Name()) {
			n++
		}
	}
	return n
}

func isPID(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// loadAndThreads reads /proc/loadavg: "0.52 0.58 0.59 1/602 3151", where the
// fourth field is running/total kernel scheduling entities, i.e. threads.
func parseLoadavg(data string) (load [3]float64, threads int) {
	f := strings.Fields(data)
	for i := 0; i < 3 && i < len(f); i++ {
		load[i], _ = strconv.ParseFloat(f[i], 64)
	}
	if len(f) >= 4 {
		if _, total, ok := strings.Cut(f[3], "/"); ok {
			threads, _ = strconv.Atoi(total)
		}
	}
	return load, threads
}

func parseUptime(data string) int64 {
	f := strings.Fields(data)
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return int64(v)
}
