package sysmon

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	vendorAMD    = "0x1002"
	vendorNVIDIA = "0x10de"
	vendorIntel  = "0x8086"
)

// gpuCard is a graphics card found under /sys/class/drm.
type gpuCard struct {
	id     string // card0
	dev    string // its sysfs device folder
	pci    string // 0000:01:00.0
	vendor string
	driver string
}

func (s *Sampler) gpuCards() []gpuCard {
	var out []gpuCard
	cards, _ := filepath.Glob(s.sys + "/class/drm/card[0-9]*")
	for _, c := range cards {
		base := filepath.Base(c)
		if strings.Contains(base, "-") { // card0-eDP-1 is an output, not a card
			continue
		}
		dev := c + "/device"
		real, err := filepath.EvalSymlinks(dev)
		if err != nil {
			real = dev
		}
		drv := ""
		if l, err := os.Readlink(dev + "/driver"); err == nil {
			drv = filepath.Base(l)
		}
		out = append(out, gpuCard{
			id: base, dev: dev, pci: filepath.Base(real),
			vendor: strings.ToLower(readString(dev + "/vendor")), driver: drv,
		})
	}
	return out
}

// gpuHwmon returns a card's sensor folder, if it has one.
func gpuHwmon(dev string) string {
	m, _ := filepath.Glob(dev + "/hwmon/hwmon*")
	if len(m) > 0 {
		return m[0]
	}
	return ""
}

func (s *Sampler) sampleAMD(c gpuCard) GPUSample {
	g := GPUSample{ID: c.id, Name: s.gpuName(c)}
	if v, ok := readFloat(c.dev + "/gpu_busy_percent"); ok {
		g.Usage = ptr(clamp(v, 0, 100))
	}
	if v, ok := readFloat(c.dev + "/mem_info_vram_used"); ok {
		g.VRAMUsed = uint64(v)
	}
	if v, ok := readFloat(c.dev + "/mem_info_vram_total"); ok {
		g.VRAMTotal = uint64(v)
	}
	if hw := gpuHwmon(c.dev); hw != "" {
		if v, ok := readFloat(hw + "/temp1_input"); ok {
			g.TempC = ptr(v / 1000)
		}
		if v, ok := readFloat(hw + "/power1_average"); ok {
			g.PowerW = ptr(v / 1e6)
		} else if v, ok := readFloat(hw + "/power1_input"); ok {
			g.PowerW = ptr(v / 1e6)
		}
		if v, ok := readFloat(hw + "/freq1_input"); ok {
			g.ClockMHz = ptr(v / 1e6)
		}
	}
	return g
}

func (s *Sampler) sampleIntel(c gpuCard) GPUSample {
	g := GPUSample{ID: c.id, Name: s.gpuName(c)}
	// The kernel doesn't report how busy an Intel GPU is without privileges; the clock it does.
	for _, f := range []string{"/gt_cur_freq_mhz", "/gt/gt0/rps_cur_freq_mhz", "/drm/" + c.id + "/gt_cur_freq_mhz"} {
		if v, ok := readFloat(c.dev + f); ok {
			g.ClockMHz = ptr(v)
			break
		}
	}
	return g
}

// nvidiaMemo is what is known of a card between looks, so that a card asleep
// can still be shown by name and size.
type nvidiaMemo struct {
	name  map[string]string
	total map[string]uint64
}

const smiQuery = "--query-gpu=pci.bus_id,name,utilization.gpu,memory.used,memory.total,temperature.gpu,power.draw,clocks.gr"

func runSMI(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "nvidia-smi", args...)
	hideConsole(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.Bytes(), err
}

type smiRow struct {
	pci                     string
	name                    string
	usage, temp, power, clk *float64
	memUsed, memTotal       uint64
}

// parseSMI reads `nvidia-smi --query-gpu=... --format=csv,noheader,nounits`.
// Fields a card can't report come as "[N/A]" and are left out.
func parseSMI(data string) []smiRow {
	var out []smiRow
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		f := strings.Split(line, ",")
		if len(f) < 8 {
			continue
		}
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		num := func(s string) *float64 {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil
			}
			return &v
		}
		r := smiRow{pci: strings.ToLower(f[0]), name: f[1], usage: num(f[2]), temp: num(f[5]), power: num(f[6]), clk: num(f[7])}
		if v := num(f[3]); v != nil {
			r.memUsed = uint64(*v * 1024 * 1024) // reported in MiB
		}
		if v := num(f[4]); v != nil {
			r.memTotal = uint64(*v * 1024 * 1024)
		}
		out = append(out, r)
	}
	return out
}

// pciKey reduces nvidia-smi's "00000000:01:00.0" and sysfs's "0000:01:00.0" to one form.
func pciKey(id string) string {
	id = strings.ToLower(id)
	if i := strings.Index(id, ":"); i > 4 {
		id = id[i-4:]
	}
	return id
}

// nvidiaAsleep reports whether the card has powered itself down. nvidia-smi
// would wake it, and keep it awake for as long as it is being asked.
func (s *Sampler) nvidiaAsleep(c gpuCard) bool {
	return readString(c.dev+"/power/runtime_status") == "suspended"
}

func (s *Sampler) sampleNVIDIA(cards []gpuCard) []GPUSample {
	if s.nvidia.name == nil {
		s.nvidia = nvidiaMemo{name: map[string]string{}, total: map[string]uint64{}}
	}
	awake := false
	for _, c := range cards {
		if !s.nvidiaAsleep(c) {
			awake = true
		}
	}
	rows := map[string]smiRow{}
	if awake {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		out, err := s.smi(ctx, smiQuery, "--format=csv,noheader,nounits")
		cancel()
		if err == nil {
			for _, r := range parseSMI(string(out)) {
				rows[pciKey(r.pci)] = r
			}
		}
	}

	var out []GPUSample
	for _, c := range cards {
		g := GPUSample{ID: c.id, Asleep: s.nvidiaAsleep(c)}
		if r, ok := rows[pciKey(c.pci)]; ok {
			g.Name, g.Usage, g.TempC, g.PowerW, g.ClockMHz = r.name, r.usage, r.temp, r.power, r.clk
			g.VRAMUsed, g.VRAMTotal = r.memUsed, r.memTotal
			s.nvidia.name[c.id], s.nvidia.total[c.id] = r.name, r.memTotal
		} else {
			g.Name = firstNonEmpty(s.nvidia.name[c.id], s.gpuName(c))
			g.VRAMTotal = s.nvidia.total[c.id]
			if g.Asleep {
				g.Usage = ptr(0)
			}
		}
		out = append(out, g)
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func (s *Sampler) sampleGPUs() []GPUSample {
	var out, nv []GPUSample
	var nvCards []gpuCard
	for _, c := range s.gpuCards() {
		switch {
		case c.vendor == vendorNVIDIA:
			nvCards = append(nvCards, c)
		case c.vendor == vendorAMD:
			out = append(out, s.sampleAMD(c))
		case c.vendor == vendorIntel:
			out = append(out, s.sampleIntel(c))
		}
	}
	if len(nvCards) > 0 {
		nv = s.sampleNVIDIA(nvCards)
	}
	out = append(nv, out...)
	for i := 1; i < len(out); i++ { // by card id, as the system numbers them
		for j := i; j > 0 && out[j].ID < out[j-1].ID; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
