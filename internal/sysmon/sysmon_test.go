package sysmon

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.3f, want %.3f (±%.3f)", name, got, want, tol)
	}
}

func TestParseCPUStat(t *testing.T) {
	all, cores := parseCPUStat(`cpu  100 0 50 800 40 5 5 0 0 0
cpu0 60 0 20 400 20 3 2 0 0 0
cpu1 40 0 30 400 20 2 3 0 0 0
intr 12345
`)
	// busy = everything but idle and iowait
	if all.total != 1000 || all.busy != 160 {
		t.Errorf("all: %+v", all)
	}
	if len(cores) != 2 || cores[0].busy != 85 || cores[1].total != 495 {
		t.Errorf("cores: %+v", cores)
	}
	near(t, "usage", usagePct(cpuTimes{busy: 100, total: 1000}, cpuTimes{busy: 300, total: 1400}), 50, 0.001)
	if usagePct(cpuTimes{total: 500}, cpuTimes{total: 500}) != 0 {
		t.Error("no time passed, no usage")
	}
	if usagePct(cpuTimes{busy: 10, total: 100}, cpuTimes{busy: 5, total: 90}) != 0 {
		t.Error("counters going backwards must not report usage")
	}
}

func TestParseMeminfo(t *testing.T) {
	m := parseMeminfo(`MemTotal:       16000000 kB
MemFree:         2000000 kB
MemAvailable:    9000000 kB
Buffers:          100000 kB
Cached:          5000000 kB
SwapTotal:       4000000 kB
SwapFree:        3000000 kB
HugePages_Total:       0
`)
	s := memorySample(m)
	if s.Total != 16000000*1024 || s.Available != 9000000*1024 || s.Used != 7000000*1024 {
		t.Errorf("memory: %+v", s)
	}
	if s.SwapUsed != 1000000*1024 || s.Cached != 5000000*1024 {
		t.Errorf("swap or cache: %+v", s)
	}
	// Old kernels have no MemAvailable.
	old := memorySample(map[string]uint64{"MemTotal": 1000, "MemFree": 100, "Cached": 200, "Buffers": 50})
	if old.Available != 350 || old.Used != 650 {
		t.Errorf("without MemAvailable: %+v", old)
	}
}

func TestParseLoadavgAndUptime(t *testing.T) {
	load, threads := parseLoadavg("1.54 4.79 4.38 1/2164 84957\n")
	if load != [3]float64{1.54, 4.79, 4.38} || threads != 2164 {
		t.Errorf("load %v threads %d", load, threads)
	}
	if parseUptime("12345.67 99999.00\n") != 12345 {
		t.Error("uptime")
	}
	if l, th := parseLoadavg(""); l != [3]float64{} || th != 0 {
		t.Error("empty loadavg")
	}
}

func TestParseDiskAndNet(t *testing.T) {
	d := parseDiskstats(`   8       0 sda 100 0 2000 50 200 0 4000 80 0 60 130
 259       0 nvme0n1 10 0 100 5 20 0 400 8 0 6 13
 259       1 nvme0n1p1 5 0 50 2 10 0 200 4 0 3 6
`)
	if d["sda"].readSectors != 2000 || d["sda"].writeSectors != 4000 || d["sda"].ioMillis != 60 || len(d) != 3 {
		t.Errorf("diskstats: %+v", d)
	}
	n := parseNetDev(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000 10 0 0 0 0 0 0 1000 10 0 0 0 0 0 0
enp2s0: 5000 50 0 0 0 0 0 0 9000 90 0 0 0 0 0 0
`)
	if n["enp2s0"].rx != 5000 || n["enp2s0"].tx != 9000 || len(n) != 2 {
		t.Errorf("net/dev: %+v", n)
	}
	for _, name := range []string{"loop0", "ram1", "zram0", "sr0", "dm-0", "nbd0"} {
		if wholeDisk(name) {
			t.Errorf("%s isn't a disk worth showing", name)
		}
	}
	if !wholeDisk("nvme0n1") || !wholeDisk("sda") || !wholeDisk("md0") {
		t.Error("real disks are shown")
	}
}

// fixture builds a fake /proc and /sys.
type fixture struct {
	t       *testing.T
	root    string
	now     time.Time
	sampler *Sampler
}

func newFixture(t *testing.T) *fixture {
	root := t.TempDir()
	f := &fixture{t: t, root: root, now: time.Unix(1_700_000_000, 0)}
	f.sampler = newSampler(root+"/proc", root+"/sys")
	f.sampler.now = func() time.Time { return f.now }
	f.sampler.smi = func(context.Context, ...string) ([]byte, error) { return nil, errors.New("no nvidia-smi") }

	f.write("proc/uptime", "5000.5 100.0\n")
	f.write("proc/loadavg", "0.50 0.60 0.70 2/800 999\n")
	f.write("proc/cpuinfo", "processor : 0\nmodel name\t: Test CPU 9000\ncpu MHz\t\t: 2000.000\n")
	for i, khz := range []string{"3000000", "4000000"} {
		cpu := "sys/devices/system/cpu/cpu" + string(rune('0'+i))
		f.write(cpu+"/cpufreq/scaling_cur_freq", khz)
		f.write(cpu+"/cpufreq/cpuinfo_max_freq", "5000000")
		f.write(cpu+"/topology/core_id", string(rune('0'+i)))
		f.write(cpu+"/topology/physical_package_id", "0")
	}
	f.write("sys/class/hwmon/hwmon0/name", "k10temp\n")
	f.write("sys/class/hwmon/hwmon0/temp1_input", "61500\n")
	f.write("sys/class/hwmon/hwmon1/name", "acpitz\n")
	f.write("sys/class/hwmon/hwmon1/temp1_input", "20000\n")

	f.mkdir("sys/block/nvme0n1/nvme0n1p1")
	f.mkdir("sys/block/loop0")
	f.write("sys/block/nvme0n1/size", "2000000\n")
	f.write("sys/block/nvme0n1/device/model", "Fast Disk 1TB\n")
	f.write("sys/block/nvme0n1/device/hwmon3/temp1_input", "38000\n")
	f.write("sys/block/nvme0n1/queue/rotational", "0\n")

	f.mkdir("sys/class/net/lo")
	f.mkdir("sys/class/net/enp2s0/device")
	f.mkdir("sys/class/net/docker0")
	f.mkdir("sys/class/net/wlp4s0/device")
	f.mkdir("sys/class/net/wlp4s0/wireless")
	f.write("sys/class/net/enp2s0/speed", "1000\n")

	f.counters(0)
	return f
}

func (f *fixture) path(rel string) string { return filepath.Join(f.root, rel) }

func (f *fixture) mkdir(rel string) {
	if err := os.MkdirAll(f.path(rel), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) write(rel, data string) {
	f.mkdir(filepath.Dir(rel))
	if err := os.WriteFile(f.path(rel), []byte(data), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// counters writes the moving parts for a given amount of elapsed work:
// at step n the counters have moved on by n steps.
func (f *fixture) counters(n int) {
	// cpu0 is busy for half of each step, cpu1 never; the overall line is their sum.
	b0, i0 := 100+n*50, 900+n*50
	b1, i1 := 100, 900+n*100
	f.write("proc/stat", "cpu  "+itoa(b0+b1)+" 0 0 "+itoa(i0+i1)+" 0 0 0 0 0 0\n"+
		"cpu0 "+itoa(b0)+" 0 0 "+itoa(i0)+" 0 0 0 0 0 0\n"+
		"cpu1 "+itoa(b1)+" 0 0 "+itoa(i1)+" 0 0 0 0 0 0\n")
	f.write("proc/meminfo", "MemTotal: 8000000 kB\nMemFree: 1000000 kB\nMemAvailable: 6000000 kB\nCached: 2000000 kB\nSwapTotal: 1000000 kB\nSwapFree: 1000000 kB\n")
	// 2048 sectors of 512 bytes = 1 MiB per step, and 300 ms of 1 s busy per step.
	f.write("proc/diskstats", "259 0 nvme0n1 1 0 "+itoa(n*2048)+" 1 1 0 "+itoa(n*4096)+" 1 0 "+itoa(n*300)+" 1\n"+
		"7 0 loop0 1 0 100 1 1 0 100 1 0 1 1\n")
	f.write("proc/net/dev", "Inter-|x\n face |y\n"+
		"    lo: 1 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n"+
		"enp2s0: "+itoa(n*1000)+" 1 0 0 0 0 0 0 "+itoa(n*500)+" 1 0 0 0 0 0 0\n"+
		"docker0: "+itoa(n*9999)+" 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n"+
		"wlp4s0: 0 1 0 0 0 0 0 0 0 1 0 0 0 0 0 0\n")
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestSampleOverTime(t *testing.T) {
	linuxOnly(t)
	f := newFixture(t)
	first := f.sampler.Sample()
	if first.CPU.Usage != 0 || first.Disks[0].ReadBps != 0 {
		t.Errorf("the first sample has nothing to compare with: %+v", first.CPU)
	}

	f.now = f.now.Add(time.Second)
	f.counters(1)
	s := f.sampler.Sample()

	near(t, "cpu usage", s.CPU.Usage, 25, 0.01) // overall: 50 busy of 200 total ticks across the two cores
	near(t, "cpu0", s.CPU.Cores[0], 50, 0.01)
	near(t, "cpu1", s.CPU.Cores[1], 0, 0.01)
	near(t, "freq", s.CPU.FreqMHz, 3500, 0.01)
	if s.CPU.TempC == nil || *s.CPU.TempC != 61.5 {
		t.Errorf("temp: %v", s.CPU.TempC)
	}
	if s.CPU.Threads != 800 || s.CPU.Uptime != 5000 || s.CPU.Load[0] != 0.5 {
		t.Errorf("cpu extras: %+v", s.CPU)
	}
	if s.Memory.Total != 8000000*1024 || s.Memory.Used != 2000000*1024 {
		t.Errorf("memory: %+v", s.Memory)
	}

	if len(s.Disks) != 1 || s.Disks[0].Name != "nvme0n1" {
		t.Fatalf("disks: %+v", s.Disks)
	}
	d := s.Disks[0]
	near(t, "read", d.ReadBps, 1<<20, 1)
	near(t, "write", d.WriteBps, 2<<20, 1)
	near(t, "active", d.ActivePct, 30, 0.01)
	if d.TempC == nil || *d.TempC != 38 {
		t.Errorf("disk temp: %v", d.TempC)
	}

	// Only real adapters, not loopback, bridges or the unconnected one's absence.
	if len(s.Nets) != 2 || s.Nets[0].Name != "enp2s0" || s.Nets[1].Name != "wlp4s0" {
		t.Fatalf("nets: %+v", s.Nets)
	}
	near(t, "rx", s.Nets[0].RxBps, 1000, 0.01)
	near(t, "tx", s.Nets[0].TxBps, 500, 0.01)

	// A counter that restarts must not show as a huge rate.
	f.now = f.now.Add(time.Second)
	f.counters(0)
	if r := f.sampler.Sample(); r.Nets[0].RxBps != 0 || r.Disks[0].ReadBps != 0 {
		t.Errorf("counter reset: %+v %+v", r.Nets[0], r.Disks[0])
	}
}

func TestInfo(t *testing.T) {
	linuxOnly(t)
	f := newFixture(t)
	f.write("proc/mounts", "/dev/nvme0n1p1 "+f.root+" ext4 rw 0 0\n/dev/loop3 /snap/x squashfs ro 0 0\ntmpfs /tmp tmpfs rw 0 0\n/dev/nvme0n1p1 "+f.root+" ext4 rw 0 0\n")
	f.write("proc/sys/kernel/osrelease", "6.99.1-test\n")
	i := f.sampler.Info()

	if i.CPU.Model != "Test CPU 9000" || i.CPU.Threads != 2 || i.CPU.Cores != 2 || i.CPU.MaxMHz != 5000 {
		t.Errorf("cpu: %+v", i.CPU)
	}
	if i.Host.Kernel != "6.99.1-test" {
		t.Errorf("host: %+v", i.Host)
	}
	if len(i.Disks) != 1 {
		t.Fatalf("disks: %+v", i.Disks)
	}
	d := i.Disks[0]
	if d.Model != "Fast Disk 1TB" || d.Kind != "NVMe SSD" || d.Size != 2000000*512 {
		t.Errorf("disk: %+v", d)
	}
	if len(d.Mounts) != 1 || d.Mounts[0].FS != "ext4" || d.Mounts[0].Total == 0 {
		t.Errorf("mounts (snap and tmpfs are left out, duplicates merged): %+v", d.Mounts)
	}
	if len(i.Nets) != 2 || i.Nets[0].Kind != "Ethernet" || i.Nets[0].SpeedMbps != 1000 || i.Nets[1].Kind != "Wi-Fi" {
		t.Errorf("nets: %+v", i.Nets)
	}
}

func TestUnescapeMount(t *testing.T) {
	if got := unescapeMount(`/mnt/My\040Games`); got != "/mnt/My Games" {
		t.Errorf("got %q", got)
	}
	if unescapeMount("/plain") != "/plain" {
		t.Error("plain paths are left alone")
	}
}

func TestPCIIDs(t *testing.T) {
	ids := "# comment\n1002  Advanced Micro Devices, Inc. [AMD/ATI]\n\t15bf  Phoenix1 [Radeon 760M/780M]\n\t\t1043 8a63  Subsystem\n\t7340  Navi 14\n10de  NVIDIA Corporation\n\t2d02  GB207M [GeForce RTX 5050 Max-Q / Mobile]\n"
	if got := findPCIName(ids, "1002", "15bf"); got != "Radeon 760M/780M" {
		t.Errorf("bracketed name: %q", got)
	}
	if got := findPCIName(ids, "1002", "7340"); got != "Navi 14" {
		t.Errorf("plain name: %q", got)
	}
	if got := findPCIName(ids, "10de", "2d02"); !strings.Contains(got, "RTX 5050") {
		t.Errorf("nvidia: %q", got)
	}
	if findPCIName(ids, "10de", "15bf") != "" {
		t.Error("a device id only counts under its own vendor")
	}
	if findPCIName(ids, "1002", "1043") != "" {
		t.Error("subsystem lines are not devices")
	}
}

func TestParseSMI(t *testing.T) {
	rows := parseSMI("00000000:01:00.0, NVIDIA GeForce RTX 5050 Laptop GPU, 37, 1500, 8151, 52, 24.80, 1800\n00000000:02:00.0, Other, [N/A], 0, 4096, [N/A], [N/A], [N/A]\n")
	if len(rows) != 2 {
		t.Fatalf("rows: %+v", rows)
	}
	r := rows[0]
	if r.name != "NVIDIA GeForce RTX 5050 Laptop GPU" || *r.usage != 37 || r.memUsed != 1500<<20 || r.memTotal != 8151<<20 || *r.temp != 52 || *r.power != 24.8 || *r.clk != 1800 {
		t.Errorf("row: %+v", r)
	}
	if rows[1].usage != nil || rows[1].temp != nil {
		t.Errorf("N/A must be left out: %+v", rows[1])
	}
	if pciKey("00000000:01:00.0") != pciKey("0000:01:00.0") {
		t.Error("both PCI id forms must match")
	}
}

func TestNVIDIACardAsleepIsNotWoken(t *testing.T) {
	linuxOnly(t)
	f := newFixture(t)
	f.mkdir("sys/class/drm/card0/device/power")
	f.write("sys/class/drm/card0/device/vendor", "0x10de\n")
	f.write("sys/class/drm/card0/device/device", "0x2d02\n")
	f.write("sys/class/drm/card0/device/power/runtime_status", "suspended\n")

	calls := 0
	f.sampler.smi = func(context.Context, ...string) ([]byte, error) {
		calls++
		return []byte("0000:00:00.0, NVIDIA Test GPU, 5, 100, 8000, 40, 10.0, 300\n"), nil
	}

	s := f.sampler.Sample()
	if calls != 0 {
		t.Fatal("a sleeping card must not be asked anything: it would wake up")
	}
	if len(s.GPUs) != 1 || !s.GPUs[0].Asleep || s.GPUs[0].Usage == nil || *s.GPUs[0].Usage != 0 {
		t.Errorf("asleep: %+v", s.GPUs)
	}

	// Awake: asked, and what it says is shown. The name is remembered for later.
	f.write("sys/class/drm/card0/device/power/runtime_status", "active\n")
	// card0's real PCI address is its folder name; point the sysfs path at one.
	s = f.sampler.Sample()
	if calls != 1 {
		t.Fatalf("an awake card is asked once per sample, got %d", calls)
	}
	if s.GPUs[0].Asleep {
		t.Error("awake")
	}
}

func TestAMDCard(t *testing.T) {
	linuxOnly(t)
	f := newFixture(t)
	dev := "sys/class/drm/card1/device"
	f.write(dev+"/vendor", "0x1002\n")
	f.write(dev+"/device", "0x15bf\n")
	f.write(dev+"/gpu_busy_percent", "42\n")
	f.write(dev+"/mem_info_vram_used", "505913344\n")
	f.write(dev+"/mem_info_vram_total", "536870912\n")
	f.write(dev+"/hwmon/hwmon4/temp1_input", "47000\n")
	f.write(dev+"/hwmon/hwmon4/power1_average", "8500000\n")
	f.write(dev+"/hwmon/hwmon4/freq1_input", "800000000\n")
	f.mkdir("sys/class/drm/card1-eDP-1") // an output, not a card

	s := f.sampler.Sample()
	if len(s.GPUs) != 1 {
		t.Fatalf("gpus: %+v", s.GPUs)
	}
	g := s.GPUs[0]
	if g.ID != "card1" || g.Usage == nil || *g.Usage != 42 || g.VRAMUsed != 505913344 || g.VRAMTotal != 536870912 {
		t.Errorf("gpu: %+v", g)
	}
	if *g.TempC != 47 || *g.PowerW != 8.5 || *g.ClockMHz != 800 || g.Asleep {
		t.Errorf("sensors: temp %v power %v clock %v", *g.TempC, *g.PowerW, *g.ClockMHz)
	}
}

func TestSampleSerialises(t *testing.T) {
	linuxOnly(t)
	f := newFixture(t)
	s := f.sampler.Sample()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"gpus":[]`, `"cores":[`, `"tempC":61.5`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("JSON lacks %s: %s", want, data)
		}
	}
}

// A smoke test on the real machine, run with SYSMON_REAL=1 to look at the numbers.
func TestRealMachine(t *testing.T) {
	if os.Getenv("SYSMON_REAL") == "" {
		t.Skip()
	}
	s := New()
	s.Sample()
	time.Sleep(time.Second)
	sample := s.Sample()
	data, _ := json.MarshalIndent(sample, "", " ")
	info, _ := json.MarshalIndent(s.Info(), "", " ")
	t.Logf("SAMPLE\n%s\nINFO\n%s", data, info)
}

func TestRadeonNameFromCPU(t *testing.T) {
	linuxOnly(t)
	if got := radeonInCPUName("model name\t: AMD Ryzen 7 250 w/ Radeon 780M Graphics\n"); got != "Radeon 780M Graphics" {
		t.Errorf("got %q", got)
	}
	if radeonInCPUName("model name\t: Intel(R) Core(TM) i7\n") != "" {
		t.Error("no Radeon in an Intel name")
	}

	// A codename from the PCI database is replaced; a real name is kept.
	f := newFixture(t)
	f.write("proc/cpuinfo", "model name\t: AMD Ryzen 7 250 w/ Radeon 780M Graphics\n")
	f.write("sys/class/drm/card1/device/vendor", "0x1002\n")
	f.write("sys/class/drm/card1/device/device", "0x15bf\n")
	if got := f.sampler.Sample().GPUs[0].Name; got != "AMD Radeon 780M Graphics" {
		t.Errorf("name %q", got)
	}
}

// linuxOnly skips tests that sample a made-up /proc and /sys, which Windows doesn't read.
func linuxOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows is sampled through its own interfaces")
	}
}
