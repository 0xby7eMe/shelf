package sysmon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type diskCounters struct {
	readSectors, writeSectors, ioMillis uint64
}

// parseDiskstats reads /proc/diskstats, keyed by device name.
func parseDiskstats(data string) map[string]diskCounters {
	out := map[string]diskCounters{}
	for _, line := range strings.Split(data, "\n") {
		f := strings.Fields(line)
		if len(f) < 13 {
			continue
		}
		var c diskCounters
		c.readSectors, _ = strconv.ParseUint(f[5], 10, 64)
		c.writeSectors, _ = strconv.ParseUint(f[9], 10, 64)
		c.ioMillis, _ = strconv.ParseUint(f[12], 10, 64)
		out[f[2]] = c
	}
	return out
}

// sectorSize is what /proc/diskstats counts in, whatever the disk's own sectors.
const sectorSize = 512

// wholeDisk reports whether a /sys/block entry is a disk worth showing: not a
// RAM disk, loop device, optical drive or device-mapper layer on top of one.
func wholeDisk(name string) bool {
	for _, p := range []string{"loop", "ram", "zram", "sr", "fd", "dm-", "nbd"} {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}

func (s *Sampler) blockDevices() []string {
	entries, _ := os.ReadDir(s.sys + "/block")
	var out []string
	for _, e := range entries {
		if wholeDisk(e.Name()) {
			out = append(out, e.Name())
		}
	}
	return out
}

// diskTemp finds a disk's temperature, which NVMe drives report through hwmon.
func (s *Sampler) diskTemp(name string) *float64 {
	for _, pattern := range []string{
		s.sys + "/block/" + name + "/device/hwmon*/temp1_input",
		s.sys + "/block/" + name + "/device/device/hwmon*/temp1_input", // SATA drives with drivetemp
	} {
		if m, _ := filepath.Glob(pattern); len(m) > 0 {
			if v, ok := readFloat(m[0]); ok {
				return ptr(v / 1000)
			}
		}
	}
	return nil
}

func (s *Sampler) sampleDisks(cur map[string]diskCounters, dt time.Duration) []DiskSample {
	var out []DiskSample
	for _, name := range s.blockDevices() {
		c, ok := cur[name]
		if !ok {
			continue
		}
		d := DiskSample{Name: name, TempC: s.diskTemp(name)}
		if prev, had := s.disks[name]; had && dt > 0 {
			secs := dt.Seconds()
			d.ReadBps = float64(sub(c.readSectors, prev.readSectors)*sectorSize) / secs
			d.WriteBps = float64(sub(c.writeSectors, prev.writeSectors)*sectorSize) / secs
			d.ActivePct = clamp(float64(sub(c.ioMillis, prev.ioMillis))/(secs*1000)*100, 0, 100)
		}
		out = append(out, d)
	}
	s.disks = cur
	return out
}

// sub is a-b that doesn't wrap when a counter restarts.
func sub(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
