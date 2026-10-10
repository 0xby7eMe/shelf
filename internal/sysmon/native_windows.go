package sysmon

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows calls the monitor makes that golang.org/x/sys doesn't wrap.

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	psapi    = windows.NewLazySystemDLL("psapi.dll")
	powrprof = windows.NewLazySystemDLL("powrprof.dll")
	pdh      = windows.NewLazySystemDLL("pdh.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")

	procGlobalMemoryStatusEx           = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetTickCount64                 = kernel32.NewProc("GetTickCount64")
	procGetLogicalProcessorInformation = kernel32.NewProc("GetLogicalProcessorInformation")
	procGetPerformanceInfo             = psapi.NewProc("GetPerformanceInfo")
	procCallNtPowerInformation         = powrprof.NewProc("CallNtPowerInformation")

	procPdhOpenQuery                = pdh.NewProc("PdhOpenQueryW")
	procPdhAddEnglishCounter        = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData         = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedCounterArray = pdh.NewProc("PdhGetFormattedCounterArrayW")

	procD3DKMTEnumAdapters2    = gdi32.NewProc("D3DKMTEnumAdapters2")
	procD3DKMTQueryAdapterInfo = gdi32.NewProc("D3DKMTQueryAdapterInfo")
	procD3DKMTCloseAdapter     = gdi32.NewProc("D3DKMTCloseAdapter")
)

// --- memory and uptime ---

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

func globalMemoryStatus() (memoryStatusEx, bool) {
	m := memoryStatusEx{}
	m.length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&m)))
	return m, r != 0
}

type performanceInformation struct {
	cb                uint32
	commitTotal       uintptr
	commitLimit       uintptr
	commitPeak        uintptr
	physicalTotal     uintptr
	physicalAvailable uintptr
	systemCache       uintptr
	kernelTotal       uintptr
	kernelPaged       uintptr
	kernelNonpaged    uintptr
	pageSize          uintptr
	handleCount       uint32
	processCount      uint32
	threadCount       uint32
}

func performanceInfo() (performanceInformation, bool) {
	p := performanceInformation{}
	p.cb = uint32(unsafe.Sizeof(p))
	r, _, _ := procGetPerformanceInfo.Call(uintptr(unsafe.Pointer(&p)), uintptr(p.cb))
	return p, r != 0
}

func uptimeSeconds() int64 {
	r, _, _ := procGetTickCount64.Call()
	return int64(r / 1000)
}

// --- processors ---

// systemProcessorPerformanceInformation is one logical processor's times, in
// 100 ns units. Kernel time includes idle time.
type systemProcessorPerformanceInformation struct {
	idleTime       int64
	kernelTime     int64
	userTime       int64
	dpcTime        int64
	interruptTime  int64
	interruptCount uint32
	_              uint32
}

const systemProcessorPerformanceInformationClass = 8

// processorTimes reads every logical processor's busy and total time.
func processorTimes() []cpuTimes {
	n := windows.GetActiveProcessorCount(windows.ALL_PROCESSOR_GROUPS)
	if n == 0 {
		return nil
	}
	buf := make([]systemProcessorPerformanceInformation, n)
	var ret uint32
	size := uint32(len(buf)) * uint32(unsafe.Sizeof(buf[0]))
	if windows.NtQuerySystemInformation(systemProcessorPerformanceInformationClass, unsafe.Pointer(&buf[0]), size, &ret) != nil {
		return nil
	}
	count := int(ret / uint32(unsafe.Sizeof(buf[0])))
	out := make([]cpuTimes, 0, count)
	for _, p := range buf[:count] {
		total := uint64(p.kernelTime + p.userTime)
		idle := uint64(p.idleTime)
		busy := uint64(0)
		if total > idle {
			busy = total - idle
		}
		out = append(out, cpuTimes{busy: busy, total: total})
	}
	return out
}

type processorPowerInformation struct {
	number           uint32
	maxMhz           uint32
	currentMhz       uint32
	mhzLimit         uint32
	maxIdleState     uint32
	currentIdleState uint32
}

const processorInformationLevel = 11

// processorClocks is the current and the highest clock of each logical processor, in MHz.
func processorClocks() (current, max []float64) {
	n := windows.GetActiveProcessorCount(windows.ALL_PROCESSOR_GROUPS)
	if n == 0 {
		return nil, nil
	}
	buf := make([]processorPowerInformation, n)
	size := uintptr(len(buf)) * unsafe.Sizeof(buf[0])
	r, _, _ := procCallNtPowerInformation.Call(processorInformationLevel, 0, 0, uintptr(unsafe.Pointer(&buf[0])), size)
	if r != 0 { // an NTSTATUS; 0 is success
		return nil, nil
	}
	for _, p := range buf {
		current = append(current, float64(p.currentMhz))
		max = append(max, float64(p.maxMhz))
	}
	return current, max
}

// physicalCores counts the processor cores, as opposed to logical processors.
func physicalCores() int {
	type info struct {
		mask         uintptr
		relationship uint32
		_            [4]byte
		_            [16]byte
	}
	var size uint32
	procGetLogicalProcessorInformation.Call(0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return 0
	}
	buf := make([]info, size/uint32(unsafe.Sizeof(info{}))+1)
	r, _, _ := procGetLogicalProcessorInformation.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return 0
	}
	cores := 0
	for _, e := range buf[:size/uint32(unsafe.Sizeof(info{}))] {
		if e.relationship == 0 { // RelationProcessorCore
			cores++
		}
	}
	return cores
}

// --- performance counters ---

// pdhQuery is a set of Windows performance counters, read together.
type pdhQuery struct {
	handle   uintptr
	counters map[string]uintptr
}

func openPDH(paths ...string) *pdhQuery {
	q := &pdhQuery{counters: map[string]uintptr{}}
	if procPdhOpenQuery.Find() != nil {
		return nil
	}
	if r, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&q.handle))); r != 0 {
		return nil
	}
	for _, p := range paths {
		path, err := windows.UTF16PtrFromString(p)
		if err != nil {
			continue
		}
		var c uintptr
		if r, _, _ := procPdhAddEnglishCounter.Call(q.handle, uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&c))); r == 0 {
			q.counters[p] = c
		}
	}
	if len(q.counters) == 0 {
		return nil
	}
	return q
}

func (q *pdhQuery) collect() bool {
	r, _, _ := procPdhCollectQueryData.Call(q.handle)
	return r == 0
}

type pdhFmtCounterValueItem struct {
	name   *uint16
	status uint32
	_      uint32
	value  float64
}

const (
	pdhFmtDouble   = 0x00000200
	pdhMoreData    = 0x800007D2
	pdhCstatusGood = 0 // PDH_CSTATUS_VALID_DATA
	pdhCstatusNew  = 1 // PDH_CSTATUS_NEW_DATA
)

// values reads every instance of a wildcard counter, by instance name.
func (q *pdhQuery) values(path string) map[string]float64 {
	c, ok := q.counters[path]
	if !ok {
		return nil
	}
	var size, count uint32
	r, _, _ := procPdhGetFormattedCounterArray.Call(c, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if uint32(r) != pdhMoreData || size == 0 {
		return nil
	}
	buf := make([]byte, size)
	r, _, _ = procPdhGetFormattedCounterArray.Call(c, pdhFmtDouble, uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return nil
	}
	items := unsafe.Slice((*pdhFmtCounterValueItem)(unsafe.Pointer(&buf[0])), count)
	out := make(map[string]float64, count)
	for _, it := range items {
		if it.status != pdhCstatusGood && it.status != pdhCstatusNew {
			continue
		}
		out[windows.UTF16PtrToString(it.name)] = it.value
	}
	return out
}

// --- graphics adapters ---

type luid struct {
	low  uint32
	high int32
}

// pdhKey is how performance counter instances name an adapter: "luid_0x00000000_0x0000d3a5".
func (l luid) pdhKey() string {
	return fmt.Sprintf("luid_0x%08x_0x%08x", uint32(l.high), l.low)
}

type d3dkmtAdapterInfo struct {
	handle                  uint32
	luid                    luid
	numOfSources            uint32
	precisePresentPreferred int32
}

type d3dkmtEnumAdapters2 struct {
	numAdapters uint32
	_           uint32
	adapters    *d3dkmtAdapterInfo
}

type d3dkmtQueryAdapterInfo struct {
	handle uint32
	typ    uint32
	data   unsafe.Pointer
	size   uint32
	_      uint32
}

const (
	kmtqaiGetSegmentSize      = 3
	kmtqaiAdapterAddress      = 6
	kmtqaiAdapterRegistryInfo = 8
	kmtqaiAdapterType         = 15

	adapterTypeSoftwareDevice = 1 << 2
)

// adapter is a graphics card as Windows lists it.
type adapter struct {
	luid     luid
	name     string
	vram     uint64 // dedicated memory
	bus      uint32 // PCI bus, to match nvidia-smi's rows
	device   uint32
	function uint32
	hasPCI   bool
}

func (a adapter) id() string { return a.luid.pdhKey() }

func (a adapter) vendor() string {
	n := strings.ToLower(a.name)
	switch {
	case strings.Contains(n, "nvidia"):
		return vendorNVIDIA
	case strings.Contains(n, "amd") || strings.Contains(n, "radeon"):
		return vendorAMD
	case strings.Contains(n, "intel"):
		return vendorIntel
	}
	return ""
}

func queryAdapter(handle uint32, typ uint32, out unsafe.Pointer, size uintptr) bool {
	q := d3dkmtQueryAdapterInfo{handle: handle, typ: typ, data: out, size: uint32(size)}
	r, _, _ := procD3DKMTQueryAdapterInfo.Call(uintptr(unsafe.Pointer(&q)))
	return r == 0
}

// adapters lists the graphics cards, leaving out Windows' software renderer.
func adapters() []adapter {
	if procD3DKMTEnumAdapters2.Find() != nil {
		return nil
	}
	var e d3dkmtEnumAdapters2
	if r, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&e))); r != 0 || e.numAdapters == 0 {
		return nil
	}
	list := make([]d3dkmtAdapterInfo, e.numAdapters)
	e.adapters = &list[0]
	if r, _, _ := procD3DKMTEnumAdapters2.Call(uintptr(unsafe.Pointer(&e))); r != 0 {
		return nil
	}

	var out []adapter
	seen := map[luid]bool{}
	for _, info := range list[:e.numAdapters] {
		a, ok := describeAdapter(info)
		procD3DKMTCloseAdapter.Call(uintptr(unsafe.Pointer(&info.handle)))
		if ok && !seen[a.luid] {
			seen[a.luid] = true
			out = append(out, a)
		}
	}
	return out
}

func describeAdapter(info d3dkmtAdapterInfo) (adapter, bool) {
	a := adapter{luid: info.luid}

	var kind uint32
	if queryAdapter(info.handle, kmtqaiAdapterType, unsafe.Pointer(&kind), unsafe.Sizeof(kind)) && kind&adapterTypeSoftwareDevice != 0 {
		return a, false
	}
	var reg struct {
		adapterString [260]uint16
		biosString    [260]uint16
		dacType       [260]uint16
		chipType      [260]uint16
	}
	if queryAdapter(info.handle, kmtqaiAdapterRegistryInfo, unsafe.Pointer(&reg), unsafe.Sizeof(reg)) {
		a.name = strings.TrimSpace(windows.UTF16ToString(reg.adapterString[:]))
	}
	if a.name == "" || strings.Contains(a.name, "Microsoft Basic Render") {
		return a, false
	}
	var seg struct{ dedicatedVideo, dedicatedSystem, sharedSystem uint64 }
	if queryAdapter(info.handle, kmtqaiGetSegmentSize, unsafe.Pointer(&seg), unsafe.Sizeof(seg)) {
		a.vram = seg.dedicatedVideo
	}
	var addr struct{ bus, device, function uint32 }
	if queryAdapter(info.handle, kmtqaiAdapterAddress, unsafe.Pointer(&addr), unsafe.Sizeof(addr)) {
		a.bus, a.device, a.function, a.hasPCI = addr.bus, addr.device, addr.function, true
	}
	return a, true
}

// pciAddress is the adapter's address as nvidia-smi writes it, reduced by pciKey.
func (a adapter) pciAddress() string {
	if !a.hasPCI {
		return ""
	}
	return pciKey(fmt.Sprintf("00000000:%02x:%02x.%x", a.bus, a.device, a.function))
}

// --- disks ---

const (
	ioctlDiskPerformance            = 0x00070020
	ioctlDiskGetDriveGeometryEx     = 0x000700A0
	ioctlStorageQueryProperty       = 0x002D1400
	ioctlVolumeGetVolumeDiskExtents = 0x00560000

	storageDeviceProperty            = 0
	storageDeviceSeekPenaltyProperty = 7
	busTypeNvme                      = 0x11
	busTypeUsb                       = 0x07
	busTypeSd                        = 0x0C
	busTypeMmc                       = 0x0D
)

type diskPerformance struct {
	bytesRead           int64
	bytesWritten        int64
	readTime            int64
	writeTime           int64
	idleTime            int64
	readCount           uint32
	writeCount          uint32
	queueDepth          uint32
	splitCount          uint32
	queryTime           int64
	storageDeviceNumber uint32
	storageManagerName  [8]uint16
}

// openDevice opens a disk or volume only to ask it questions, which needs no
// administrator rights.
func openDevice(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(p, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

func ioctl(h windows.Handle, code uint32, in []byte, out []byte) (int, bool) {
	var inPtr *byte
	if len(in) > 0 {
		inPtr = &in[0]
	}
	var n uint32
	err := windows.DeviceIoControl(h, code, inPtr, uint32(len(in)), &out[0], uint32(len(out)), &n, nil)
	return int(n), err == nil
}

// physicalDisks lists the numbers of the disks Windows has, up to a sensible count.
func physicalDisks() []int {
	var out []int
	for i := 0; i < 32; i++ {
		h, err := openDevice(fmt.Sprintf(`\\.\PhysicalDrive%d`, i))
		if err != nil {
			continue
		}
		windows.CloseHandle(h)
		out = append(out, i)
	}
	return out
}

func readDiskPerformance(n int) (diskPerformance, bool) {
	var perf diskPerformance
	h, err := openDevice(fmt.Sprintf(`\\.\PhysicalDrive%d`, n))
	if err != nil {
		return perf, false
	}
	defer windows.CloseHandle(h)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&perf)), unsafe.Sizeof(perf))
	_, ok := ioctl(h, ioctlDiskPerformance, nil, buf)
	return perf, ok
}

// describeDisk reads a disk's model, size and kind.
func describeDisk(n int) (model string, size uint64, kind string) {
	h, err := openDevice(fmt.Sprintf(`\\.\PhysicalDrive%d`, n))
	if err != nil {
		return "", 0, ""
	}
	defer windows.CloseHandle(h)

	geo := make([]byte, 256)
	if got, ok := ioctl(h, ioctlDiskGetDriveGeometryEx, nil, geo); ok && got >= 32 {
		size = *(*uint64)(unsafe.Pointer(&geo[24])) // after DISK_GEOMETRY
	}

	query := make([]byte, 12) // STORAGE_PROPERTY_QUERY: id, PropertyStandardQuery, padding
	desc := make([]byte, 1024)
	bus := uint32(0)
	if got, ok := ioctl(h, ioctlStorageQueryProperty, query, desc); ok && got >= 32 {
		product := *(*uint32)(unsafe.Pointer(&desc[16]))
		vendor := *(*uint32)(unsafe.Pointer(&desc[12]))
		bus = *(*uint32)(unsafe.Pointer(&desc[28]))
		model = strings.TrimSpace(cString(desc, vendor) + " " + cString(desc, product))
	}

	penalty := make([]byte, 12)
	query[0] = storageDeviceSeekPenaltyProperty
	seeks, known := false, false
	if got, ok := ioctl(h, ioctlStorageQueryProperty, query, penalty); ok && got >= 9 {
		seeks, known = penalty[8] != 0, true
	}
	switch {
	case bus == busTypeNvme:
		kind = "NVMe SSD"
	case known && seeks:
		kind = "HDD"
	default:
		kind = "SSD"
	}
	return model, size, kind
}

// cString reads a NUL-terminated string at an offset of a descriptor; 0 means none.
func cString(buf []byte, off uint32) string {
	if off == 0 || int(off) >= len(buf) {
		return ""
	}
	end := int(off)
	for end < len(buf) && buf[end] != 0 {
		end++
	}
	return strings.TrimSpace(string(buf[off:end]))
}

// volumeDisk is the disk a drive letter's volume starts on.
func volumeDisk(letter byte) (int, bool) {
	h, err := openDevice(`\\.\` + string(letter) + `:`)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h)
	buf := make([]byte, 256)
	got, ok := ioctl(h, ioctlVolumeGetVolumeDiskExtents, nil, buf)
	if !ok || got < 32 || *(*uint32)(unsafe.Pointer(&buf[0])) == 0 {
		return 0, false
	}
	return int(*(*uint32)(unsafe.Pointer(&buf[8]))), true
}
