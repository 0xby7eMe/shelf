package sysmon

import (
	"os"
	"strings"
)

// gpuName turns a card's PCI ids into a marketing name using the system's
// pci.ids database, preferring the part in brackets ("Radeon 760M/780M").
func (s *Sampler) gpuName(c gpuCard) string {
	vendor := strings.TrimPrefix(c.vendor, "0x")
	device := strings.TrimPrefix(strings.ToLower(readString(c.dev+"/device")), "0x")
	name := lookupPCI(vendor, device)
	if vendor == "1002" && !strings.Contains(name, "Radeon") {
		// An AMD processor's built-in graphics are named in the processor's own
		// name ("... w/ Radeon 780M Graphics"), where the PCI database has a codename.
		if m := radeonInCPUName(s.readProc("cpuinfo")); m != "" {
			return "AMD " + m
		}
	}
	if name != "" {
		return vendorLabel(vendor) + " " + name
	}
	return vendorLabel(vendor) + " graphics"
}

func vendorLabel(vendor string) string {
	switch vendor {
	case "1002":
		return "AMD"
	case "10de":
		return "NVIDIA"
	case "8086":
		return "Intel"
	}
	return "GPU"
}

var pciIDPaths = []string{"/usr/share/hwdata/pci.ids", "/usr/share/misc/pci.ids", "/usr/share/pci.ids"}

func lookupPCI(vendor, device string) string {
	for _, p := range pciIDPaths {
		if data, err := os.ReadFile(p); err == nil {
			return findPCIName(string(data), vendor, device)
		}
	}
	return ""
}

// findPCIName looks a device up in pci.ids text: vendor lines start in the
// first column, their devices are indented by a tab.
func findPCIName(data, vendor, device string) string {
	inVendor := false
	for _, line := range strings.Split(data, "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if line[0] != '\t' {
			inVendor = strings.HasPrefix(line, vendor+"  ")
			continue
		}
		if inVendor && strings.HasPrefix(line, "\t"+device+"  ") && !strings.HasPrefix(line, "\t\t") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "\t"+device))
			if i, j := strings.Index(name, "["), strings.LastIndex(name, "]"); i >= 0 && j > i {
				return name[i+1 : j]
			}
			return name
		}
	}
	return ""
}

// radeonInCPUName returns the "Radeon ... Graphics" part of a processor's model name.
func radeonInCPUName(cpuinfo string) string {
	for _, line := range strings.Split(cpuinfo, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
			if i := strings.Index(v, "Radeon"); i >= 0 {
				return strings.TrimSpace(v[i:])
			}
			return ""
		}
	}
	return ""
}
