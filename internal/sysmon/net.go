package sysmon

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type netCounters struct{ rx, tx uint64 }

// parseNetDev reads /proc/net/dev, keyed by interface name.
func parseNetDev(data string) map[string]netCounters {
	out := map[string]netCounters{}
	for _, line := range strings.Split(data, "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		var c netCounters
		c.rx, _ = strconv.ParseUint(f[0], 10, 64)
		c.tx, _ = strconv.ParseUint(f[8], 10, 64)
		out[strings.TrimSpace(name)] = c
	}
	return out
}

// physicalNIC reports whether an interface is a real adapter, which is one with
// hardware behind it, as opposed to loopback, bridges, containers and tunnels.
func (s *Sampler) physicalNIC(name string) bool {
	if name == "lo" {
		return false
	}
	_, err := os.Stat(s.sys + "/class/net/" + name + "/device")
	return err == nil
}

func (s *Sampler) sampleNets(cur map[string]netCounters, dt time.Duration) []NetSample {
	var out []NetSample
	for name, c := range cur {
		if !s.physicalNIC(name) {
			continue
		}
		n := NetSample{Name: name}
		if prev, had := s.nets[name]; had && dt > 0 {
			secs := dt.Seconds()
			n.RxBps = float64(sub(c.rx, prev.rx)) / secs
			n.TxBps = float64(sub(c.tx, prev.tx)) / secs
		}
		out = append(out, n)
	}
	s.nets = cur
	sortNets(out)
	return out
}

func sortNets(n []NetSample) {
	for i := 1; i < len(n); i++ {
		for j := i; j > 0 && n[j].Name < n[j-1].Name; j-- {
			n[j], n[j-1] = n[j-1], n[j]
		}
	}
}
