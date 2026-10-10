package sysmon

import (
	"os"
	"runtime"
	"sort"
	"strconv"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func (s *Sampler) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, _ := os.Hostname()
	info := Info{
		Host:  HostInfo{Hostname: host, OS: windowsName(), Kernel: windowsVersion()},
		CPU:   windowsCPU(),
		Disks: windowsDisks(),
		Nets:  windowsNets(),
		GPUs:  []GPUInfo{},
	}
	if m, ok := globalMemoryStatus(); ok {
		info.Memory.Total = m.totalPhys
		if m.totalPageFile > m.totalPhys {
			info.Memory.SwapTotal = m.totalPageFile - m.totalPhys
		}
	}
	for _, a := range s.gpuAdapters() {
		info.GPUs = append(info.GPUs, GPUInfo{ID: a.id(), Name: a.name, VRAMTotal: a.vram})
	}
	return info
}

func ntVersionKey() (registry.Key, error) {
	return registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
}

// windowsName is the edition and release, such as "Windows 11 Pro 24H2".
// Windows 11 still calls itself Windows 10 in the registry; the build tells.
func windowsName() string {
	k, err := ntVersionKey()
	if err != nil {
		return "Windows"
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProductName")
	if name == "" {
		name = "Windows"
	}
	if v := windows.RtlGetVersion(); v != nil && v.BuildNumber >= 22000 && len(name) >= 10 && name[:10] == "Windows 10" {
		name = "Windows 11" + name[10:]
	}
	if release, _, err := k.GetStringValue("DisplayVersion"); err == nil && release != "" {
		name += " " + release
	}
	return name
}

func windowsVersion() string {
	v := windows.RtlGetVersion()
	if v == nil {
		return ""
	}
	return strconv.Itoa(int(v.MajorVersion)) + "." + strconv.Itoa(int(v.MinorVersion)) + "." + strconv.Itoa(int(v.BuildNumber))
}

func windowsCPU() CPUInfo {
	info := CPUInfo{Arch: runtime.GOARCH, Threads: runtime.NumCPU(), Cores: physicalCores()}
	if info.Cores == 0 {
		info.Cores = info.Threads
	}
	if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE); err == nil {
		info.Model, _, _ = k.GetStringValue("ProcessorNameString")
		if mhz, _, err := k.GetIntegerValue("~MHz"); err == nil {
			info.MaxMHz = float64(mhz)
		}
		k.Close()
	}
	if _, maxes := processorClocks(); len(maxes) > 0 {
		for _, m := range maxes {
			info.MaxMHz = max(info.MaxMHz, m)
		}
	}
	return info
}

func windowsDisks() []DiskInfo {
	mounts := map[int][]MountInfo{}
	drives, _ := windows.GetLogicalDrives()
	for i := 0; i < 26; i++ {
		if drives&(1<<i) == 0 {
			continue
		}
		letter := byte('A' + i)
		root := string(letter) + `:\`
		rootPtr, _ := windows.UTF16PtrFromString(root)
		if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
			continue
		}
		disk, ok := volumeDisk(letter)
		if !ok {
			continue
		}
		m := MountInfo{Path: root}
		fsName := make([]uint16, 64)
		if windows.GetVolumeInformation(rootPtr, nil, 0, nil, nil, nil, &fsName[0], uint32(len(fsName))) == nil {
			m.FS = windows.UTF16ToString(fsName)
		}
		var avail, total, free uint64
		if windows.GetDiskFreeSpaceEx(rootPtr, &avail, &total, &free) == nil {
			m.Total, m.Free = total, avail
		}
		mounts[disk] = append(mounts[disk], m)
	}

	var out []DiskInfo
	for _, n := range physicalDisks() {
		model, size, kind := describeDisk(n)
		out = append(out, DiskInfo{Name: diskName(n), Model: model, Size: size, Kind: kind, Mounts: mounts[n]})
	}
	return out
}

func windowsNets() []NetInfo {
	physical := physicalAdapters()
	var out []NetInfo
	for _, a := range adapterAddresses() {
		if _, ok := physical[a.Name]; !ok {
			continue
		}
		n := NetInfo{Name: a.Name, Kind: "Ethernet", Addrs: a.Addrs}
		if a.IfType == windows.IF_TYPE_IEEE80211 {
			n.Kind = "Wi-Fi"
		}
		if a.Speed > 0 && a.Speed != ^uint64(0) {
			n.SpeedMbps = int(a.Speed / 1_000_000)
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
