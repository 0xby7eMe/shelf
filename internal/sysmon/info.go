package sysmon

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Info is what doesn't change while Shelf runs: names, sizes and capabilities.
type Info struct {
	Host   HostInfo   `json:"host"`
	CPU    CPUInfo    `json:"cpu"`
	Memory MemoryInfo `json:"memory"`
	Disks  []DiskInfo `json:"disks"`
	Nets   []NetInfo  `json:"nets"`
	GPUs   []GPUInfo  `json:"gpus"`
}

type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
}

type CPUInfo struct {
	Model   string  `json:"model"`
	Cores   int     `json:"cores"`   // physical
	Threads int     `json:"threads"` // logical processors
	MaxMHz  float64 `json:"maxMHz"`
	Arch    string  `json:"arch"`
}

type MemoryInfo struct {
	Total     uint64 `json:"total"`
	SwapTotal uint64 `json:"swapTotal"`
}

type MountInfo struct {
	Path  string `json:"path"`
	FS    string `json:"fs"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

type DiskInfo struct {
	Name   string      `json:"name"`
	Model  string      `json:"model"`
	Size   uint64      `json:"size"`
	Kind   string      `json:"kind"` // "NVMe SSD", "SSD" or "HDD"
	Mounts []MountInfo `json:"mounts"`
}

type NetInfo struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"` // "Ethernet" or "Wi-Fi"
	SpeedMbps int      `json:"speedMbps"`
	Addrs     []string `json:"addrs"`
}

type GPUInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Driver    string `json:"driver"`
	VRAMTotal uint64 `json:"vramTotal"`
}

func osName() string {
	data, _ := os.ReadFile("/etc/os-release")
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(v, `"`)
		}
	}
	return "Linux"
}

func (s *Sampler) cpuInfo() CPUInfo {
	info := CPUInfo{Arch: runtime.GOARCH}
	for _, line := range strings.Split(s.readProc("cpuinfo"), "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			info.Model = strings.TrimSpace(v)
			break
		}
	}
	// Cores are the distinct (package, core) pairs the kernel reports.
	dirs, _ := filepath.Glob(s.sys + "/devices/system/cpu/cpu[0-9]*")
	cores := map[string]bool{}
	for _, d := range dirs {
		info.Threads++
		cores[readString(d+"/topology/physical_package_id")+"/"+readString(d+"/topology/core_id")] = true
	}
	info.Cores = len(cores)
	if info.Threads == 0 {
		info.Threads = runtime.NumCPU()
		info.Cores = info.Threads
	}
	for _, d := range dirs {
		if v, ok := readFloat(d + "/cpufreq/cpuinfo_max_freq"); ok && v/1000 > info.MaxMHz {
			info.MaxMHz = v / 1000
		}
	}
	return info
}

// diskOf finds the disk a partition belongs to, or the disk itself.
func (s *Sampler) diskOf(dev string) string {
	for _, d := range s.blockDevices() {
		if dev == d {
			return d
		}
		if _, err := os.Stat(s.sys + "/block/" + d + "/" + dev); err == nil {
			return d
		}
	}
	return ""
}

// unescapeMount undoes the octal escapes /proc/mounts uses for spaces and the like.
func unescapeMount(p string) string {
	if !strings.Contains(p, `\`) {
		return p
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '\\' && i+3 < len(p) {
			if n, err := strconv.ParseUint(p[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(p[i])
	}
	return b.String()
}

func (s *Sampler) netInfo() []NetInfo {
	entries, _ := os.ReadDir(s.sys + "/class/net")
	var out []NetInfo
	for _, e := range entries {
		name := e.Name()
		if !s.physicalNIC(name) {
			continue
		}
		n := NetInfo{Name: name, Kind: "Ethernet"}
		if _, err := os.Stat(s.sys + "/class/net/" + name + "/wireless"); err == nil {
			n.Kind = "Wi-Fi"
		}
		if v, ok := readFloat(s.sys + "/class/net/" + name + "/speed"); ok && v > 0 {
			n.SpeedMbps = int(v)
		}
		if ifc, err := net.InterfaceByName(name); err == nil {
			if addrs, err := ifc.Addrs(); err == nil {
				for _, a := range addrs {
					if ip, _, err := net.ParseCIDR(a.String()); err == nil && !ip.IsLinkLocalUnicast() {
						n.Addrs = append(n.Addrs, ip.String())
					}
				}
			}
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Sampler) gpuInfo() []GPUInfo {
	var out []GPUInfo
	for _, c := range s.gpuCards() {
		if c.vendor != vendorAMD && c.vendor != vendorNVIDIA && c.vendor != vendorIntel {
			continue
		}
		g := GPUInfo{ID: c.id, Name: s.gpuName(c), Driver: c.driver}
		if v, ok := readFloat(c.dev + "/mem_info_vram_total"); ok {
			g.VRAMTotal = uint64(v)
		}
		out = append(out, g)
	}
	return out
}
