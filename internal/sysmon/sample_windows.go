package sysmon

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// winState is what Windows needs kept between samples: the disks' and
// adapters' counters, and the performance counter query for the graphics cards.
type winState struct {
	disks    map[int]diskPerformance
	nets     map[string]netCounters
	gpus     []adapter
	gpusAt   time.Time
	pdh      *pdhQuery
	pdhTried bool
}

const (
	gpuEngineCounter = `\GPU Engine(*)\Utilization Percentage`
	gpuMemoryCounter = `\GPU Adapter Memory(*)\Dedicated Usage`
)

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
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

	cores := processorTimes()
	var all cpuTimes
	for _, c := range cores {
		all.busy += c.busy
		all.total += c.total
	}
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
	if cur, _ := processorClocks(); len(cur) > 0 {
		sum := 0.0
		for _, v := range cur {
			sum += v
		}
		out.CPU.FreqMHz = sum / float64(len(cur))
	}
	if p, ok := performanceInfo(); ok {
		out.CPU.Processes, out.CPU.Threads = int(p.processCount), int(p.threadCount)
		out.Memory.Cached = uint64(p.systemCache) * uint64(p.pageSize)
	}
	out.CPU.Uptime = uptimeSeconds()
	// Windows has no load average, and only administrators can read the processor's temperature.

	if m, ok := globalMemoryStatus(); ok {
		out.Memory.Total = m.totalPhys
		out.Memory.Available = m.availPhys
		out.Memory.Free = m.availPhys
		out.Memory.Used = m.totalPhys - m.availPhys
		// The commit limit is memory plus the page file.
		if m.totalPageFile > m.totalPhys {
			out.Memory.SwapTotal = m.totalPageFile - m.totalPhys
			committed := m.totalPageFile - m.availPageFile
			if inRAM := m.totalPhys - m.availPhys; committed > inRAM {
				out.Memory.SwapUsed = min(committed-inRAM, out.Memory.SwapTotal)
			}
		}
	}

	out.Disks = s.sampleWindowsDisks(dt)
	out.Nets = s.sampleWindowsNets(dt)
	out.GPUs = s.sampleWindowsGPUs()
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

func diskName(n int) string { return fmt.Sprintf("Disk %d", n) }

func (s *Sampler) sampleWindowsDisks(dt time.Duration) []DiskSample {
	cur := map[int]diskPerformance{}
	var out []DiskSample
	for _, n := range physicalDisks() {
		perf, ok := readDiskPerformance(n)
		if !ok {
			continue
		}
		cur[n] = perf
		d := DiskSample{Name: diskName(n)}
		if prev, had := s.win.disks[n]; had && dt > 0 {
			secs := dt.Seconds()
			d.ReadBps = float64(sub(uint64(perf.bytesRead), uint64(prev.bytesRead))) / secs
			d.WriteBps = float64(sub(uint64(perf.bytesWritten), uint64(prev.bytesWritten))) / secs
			// Both times are in 100 ns units; busy is whatever wasn't idle.
			if query := sub(uint64(perf.queryTime), uint64(prev.queryTime)); query > 0 {
				idle := float64(sub(uint64(perf.idleTime), uint64(prev.idleTime)))
				d.ActivePct = clamp(100-idle/float64(query)*100, 0, 100)
			}
		}
		out = append(out, d)
	}
	s.win.disks = cur
	return out
}

// physicalAdapters lists the network adapters that are real hardware, by
// name, with their row of counters.
func physicalAdapters() map[string]windows.MibIfRow2 {
	out := map[string]windows.MibIfRow2{}
	for _, a := range adapterAddresses() {
		if a.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK || a.IfType == windows.IF_TYPE_TUNNEL {
			continue
		}
		row := windows.MibIfRow2{InterfaceIndex: a.IfIndex}
		if windows.GetIfEntry2Ex(windows.MibIfEntryNormal, &row) != nil {
			continue
		}
		if row.InterfaceAndOperStatusFlags&1 == 0 { // HardwareInterface
			continue
		}
		out[a.Name] = row
	}
	return out
}

// adapterAddresses copies out the adapters Windows lists, with what is needed of them.
func adapterAddresses() []adapterEntry {
	size := uint32(16 << 10)
	for range 3 {
		buf := make([]byte, size)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_SKIP_ANYCAST|windows.GAA_FLAG_SKIP_MULTICAST|windows.GAA_FLAG_SKIP_DNS_SERVER, 0, first, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil
		}
		var out []adapterEntry
		for a := first; a != nil; a = a.Next {
			e := adapterEntry{IfIndex: a.IfIndex, IfType: a.IfType, Name: windows.UTF16PtrToString(a.FriendlyName), Speed: a.TransmitLinkSpeed}
			for u := a.FirstUnicastAddress; u != nil; u = u.Next {
				if ip := u.Address.IP(); ip != nil && !ip.IsLinkLocalUnicast() {
					e.Addrs = append(e.Addrs, ip.String())
				}
			}
			out = append(out, e)
		}
		return out
	}
	return nil
}

// adapterEntry is a network adapter, copied out of Windows' list.
type adapterEntry struct {
	IfIndex uint32
	IfType  uint32
	Name    string
	Speed   uint64 // bits per second
	Addrs   []string
}

func (s *Sampler) sampleWindowsNets(dt time.Duration) []NetSample {
	cur := map[string]netCounters{}
	var out []NetSample
	for name, row := range physicalAdapters() {
		c := netCounters{rx: row.InOctets, tx: row.OutOctets}
		cur[name] = c
		n := NetSample{Name: name}
		if prev, had := s.win.nets[name]; had && dt > 0 {
			secs := dt.Seconds()
			n.RxBps = float64(sub(c.rx, prev.rx)) / secs
			n.TxBps = float64(sub(c.tx, prev.tx)) / secs
		}
		out = append(out, n)
	}
	s.win.nets = cur
	sortNets(out)
	return out
}

// gpuAdapters is the list of graphics cards, looked up again now and then
// rather than every second.
func (s *Sampler) gpuAdapters() []adapter {
	if s.win.gpus == nil || time.Since(s.win.gpusAt) > time.Minute {
		s.win.gpus = adapters()
		s.win.gpusAt = time.Now()
	}
	return s.win.gpus
}

// sampleWindowsGPUs reads how busy each card is from Windows' own counters, as
// Task Manager does, and asks nvidia-smi for what NVIDIA cards say beyond that.
func (s *Sampler) sampleWindowsGPUs() []GPUSample {
	list := s.gpuAdapters()
	if len(list) == 0 {
		return nil
	}
	if !s.win.pdhTried {
		s.win.pdhTried = true
		s.win.pdh = openPDH(gpuEngineCounter, gpuMemoryCounter)
	}
	usage, memory := map[string]float64{}, map[string]float64{}
	if q := s.win.pdh; q != nil && q.collect() {
		usage = engineUsage(q.values(gpuEngineCounter))
		for inst, v := range q.values(gpuMemoryCounter) {
			memory[adapterOf(inst)] += v
		}
	}

	var smi map[string]smiRow
	for _, a := range list {
		if a.vendor() == vendorNVIDIA {
			smi = s.windowsSMI()
			break
		}
	}

	out := make([]GPUSample, 0, len(list))
	for _, a := range list {
		g := GPUSample{ID: a.id(), Name: a.name, VRAMTotal: a.vram}
		if v, ok := usage[a.luid.pdhKey()]; ok {
			g.Usage = ptr(clamp(v, 0, 100))
		}
		if v, ok := memory[a.luid.pdhKey()]; ok {
			g.VRAMUsed = uint64(v)
		}
		if r, ok := smi[a.pciAddress()]; ok && a.pciAddress() != "" {
			g.TempC, g.PowerW, g.ClockMHz = r.temp, r.power, r.clk
			if r.usage != nil {
				g.Usage = r.usage
			}
			if r.memTotal > 0 {
				g.VRAMUsed, g.VRAMTotal = r.memUsed, r.memTotal
			}
		}
		out = append(out, g)
	}
	return out
}

func (s *Sampler) windowsSMI() map[string]smiRow {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, err := s.smi(ctx, smiQuery, "--format=csv,noheader,nounits")
	if err != nil {
		return nil
	}
	rows := map[string]smiRow{}
	for _, r := range parseSMI(string(data)) {
		rows[pciKey(r.pci)] = r
	}
	return rows
}
