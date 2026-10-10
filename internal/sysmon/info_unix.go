//go:build !windows

package sysmon

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

func (s *Sampler) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, _ := os.Hostname()
	return Info{
		Host:   HostInfo{Hostname: host, OS: osName(), Kernel: readString(s.proc + "/sys/kernel/osrelease")},
		CPU:    s.cpuInfo(),
		Memory: MemoryInfo{Total: parseMeminfo(s.readProc("meminfo"))["MemTotal"], SwapTotal: parseMeminfo(s.readProc("meminfo"))["SwapTotal"]},
		Disks:  s.diskInfo(),
		Nets:   s.netInfo(),
		GPUs:   s.gpuInfo(),
	}
}

func (s *Sampler) diskInfo() []DiskInfo {
	mounts := s.mounts()
	var out []DiskInfo
	for _, name := range s.blockDevices() {
		sectors, _ := readFloat(s.sys + "/block/" + name + "/size")
		d := DiskInfo{
			Name:  name,
			Model: readString(s.sys + "/block/" + name + "/device/model"),
			Size:  uint64(sectors) * sectorSize,
		}
		rotational := readString(s.sys+"/block/"+name+"/queue/rotational") == "1"
		switch {
		case strings.HasPrefix(name, "nvme"):
			d.Kind = "NVMe SSD"
		case rotational:
			d.Kind = "HDD"
		default:
			d.Kind = "SSD"
		}
		for _, m := range mounts {
			if m.disk == name {
				d.Mounts = append(d.Mounts, m.MountInfo)
			}
		}
		out = append(out, d)
	}
	return out
}

type mountEntry struct {
	MountInfo
	disk string
}

// mounts lists the filesystems on real disks, with their space.
func (s *Sampler) mounts() []mountEntry {
	skip := []string{"/var/lib/docker", "/var/lib/containers", "/snap", "/run", "/sys", "/proc", "/dev"}
	seen := map[string]bool{}
	var out []mountEntry
	for _, line := range strings.Split(s.readProc("mounts"), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || !strings.HasPrefix(f[0], "/dev/") {
			continue
		}
		path := unescapeMount(f[1])
		if seen[path] {
			continue
		}
		skipIt := false
		for _, p := range skip {
			if path == p || strings.HasPrefix(path, p+"/") {
				skipIt = true
			}
		}
		if skipIt {
			continue
		}
		disk := s.diskOf(filepath.Base(f[0]))
		if disk == "" {
			continue
		}
		var st syscall.Statfs_t
		if err := syscall.Statfs(path, &st); err != nil {
			continue
		}
		seen[path] = true
		out = append(out, mountEntry{
			MountInfo: MountInfo{
				Path: path, FS: f[2],
				Total: st.Blocks * uint64(st.Bsize), Free: st.Bavail * uint64(st.Bsize),
			},
			disk: disk,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
