//go:build !windows

package sysmon

import (
	"os/exec"
	"time"
)

// winState is empty: Linux keeps its counters in /proc.
type winState struct{}

func hideConsole(*exec.Cmd) {}

// Sample takes a sample of the machine. Rates cover the time since the last call.
func (s *Sampler) Sample() Sample {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	first := s.last.IsZero()
	var dt time.Duration
	if !first {
		dt = now.Sub(s.last)
	}
	s.last = now

	out := Sample{Time: now.UnixMilli()}

	all, cores := parseCPUStat(s.readProc("stat"))
	out.CPU.Cores = make([]float64, len(cores))
	if !first {
		out.CPU.Usage = usagePct(s.cpu, all)
		for i := range cores {
			if i < len(s.cores) {
				out.CPU.Cores[i] = usagePct(s.cores[i], cores[i])
			}
		}
	}
	s.cpu, s.cores = all, cores
	out.CPU.FreqMHz = s.cpuFrequency()
	out.CPU.TempC = s.cpuTemp()
	out.CPU.Processes = s.countProcesses()
	out.CPU.Load, out.CPU.Threads = parseLoadavg(s.readProc("loadavg"))
	out.CPU.Uptime = parseUptime(s.readProc("uptime"))

	out.Memory = memorySample(parseMeminfo(s.readProc("meminfo")))
	out.Disks = s.sampleDisks(parseDiskstats(s.readProc("diskstats")), dt)
	out.Nets = s.sampleNets(parseNetDev(s.readProc("net/dev")), dt)
	out.GPUs = s.sampleGPUs()
	if out.Disks == nil {
		out.Disks = []DiskSample{}
	}
	if out.Nets == nil {
		out.Nets = []NetSample{}
	}
	if out.GPUs == nil {
		out.GPUs = []GPUSample{}
	}
	return out
}
