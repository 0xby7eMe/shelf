// Package sysmon reads what the machine is doing, from /proc and /sys: processor,
// memory, disks, network, graphics cards and processes. It is what the
// performance view is made of, in the manner of a task manager.
package sysmon

import (
	"context"
	"os"
	"sync"
	"time"
)

// Sample is the state of the machine at one moment. Rates are measured over
// the time since the previous sample, so the first one reports zeroes.
type Sample struct {
	Time   int64        `json:"time"` // unix milliseconds
	CPU    CPUSample    `json:"cpu"`
	Memory MemorySample `json:"memory"`
	Disks  []DiskSample `json:"disks"`
	Nets   []NetSample  `json:"nets"`
	GPUs   []GPUSample  `json:"gpus"`
}

type CPUSample struct {
	Usage     float64    `json:"usage"`     // percent of all logical processors
	Cores     []float64  `json:"cores"`     // percent per logical processor
	FreqMHz   float64    `json:"freqMHz"`   // average over the cores, 0 if unknown
	TempC     *float64   `json:"tempC"`     // package temperature, null if unknown
	Processes int        `json:"processes"` // running programs
	Threads   int        `json:"threads"`
	Load      [3]float64 `json:"load"`   // 1, 5 and 15 minute load averages
	Uptime    int64      `json:"uptime"` // seconds
}

type MemorySample struct {
	Total     uint64 `json:"total"` // bytes
	Used      uint64 `json:"used"`  // total minus what programs could still get
	Available uint64 `json:"available"`
	Free      uint64 `json:"free"`
	Cached    uint64 `json:"cached"`
	Buffers   uint64 `json:"buffers"`
	SwapTotal uint64 `json:"swapTotal"`
	SwapUsed  uint64 `json:"swapUsed"`
}

type DiskSample struct {
	Name      string   `json:"name"`
	ReadBps   float64  `json:"readBps"`
	WriteBps  float64  `json:"writeBps"`
	ActivePct float64  `json:"activePct"` // share of time the disk was busy
	TempC     *float64 `json:"tempC"`
}

type NetSample struct {
	Name  string  `json:"name"`
	RxBps float64 `json:"rxBps"`
	TxBps float64 `json:"txBps"`
}

type GPUSample struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Usage     *float64 `json:"usage"` // percent, null if the driver doesn't say
	VRAMUsed  uint64   `json:"vramUsed"`
	VRAMTotal uint64   `json:"vramTotal"`
	TempC     *float64 `json:"tempC"`
	PowerW    *float64 `json:"powerW"`
	ClockMHz  *float64 `json:"clockMHz"`
	// Asleep is set for a card that is powered down to save energy. Asking it
	// anything would wake it, so it isn't asked.
	Asleep bool `json:"asleep"`
}

// Sampler takes samples. It remembers the previous counters, so use one for
// the life of a view.
type Sampler struct {
	proc, sys string
	now       func() time.Time
	// smi runs nvidia-smi with the given arguments; a variable for tests.
	smi func(ctx context.Context, args ...string) ([]byte, error)

	mu     sync.Mutex
	last   time.Time
	cpu    cpuTimes
	cores  []cpuTimes
	disks  map[string]diskCounters
	nets   map[string]netCounters
	nvidia nvidiaMemo
}

// New returns a Sampler for this machine.
func New() *Sampler {
	return newSampler("/proc", "/sys")
}

func newSampler(proc, sys string) *Sampler {
	return &Sampler{
		proc: proc, sys: sys,
		now:   time.Now,
		smi:   runSMI,
		disks: map[string]diskCounters{},
		nets:  map[string]netCounters{},
	}
}

func (s *Sampler) readProc(name string) string {
	b, _ := os.ReadFile(s.proc + "/" + name)
	return string(b)
}

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
