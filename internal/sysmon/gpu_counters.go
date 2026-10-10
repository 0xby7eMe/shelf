package sysmon

import "strings"

// Windows reports how busy a graphics card is through performance counters,
// one per process and engine. These turn them into a figure per card.

// adapterOf reduces a counter instance such as
// "pid_1234_luid_0x00000000_0x0000D3A5_phys_0_eng_3_engtype_3D" to the adapter's key.
func adapterOf(instance string) string {
	inst := strings.ToLower(instance)
	i := strings.Index(inst, "luid_")
	if i < 0 || len(inst) < i+26 {
		return ""
	}
	return inst[i : i+26]
}

// engineUsage turns the per-process, per-engine figures into one number per
// card: the busiest kind of engine (3D, copy, video), summed over processes.
func engineUsage(values map[string]float64) map[string]float64 {
	perType := map[string]map[string]float64{} // adapter -> engine type -> percent
	for inst, v := range values {
		key := adapterOf(inst)
		if key == "" {
			continue
		}
		kind := "other"
		if i := strings.Index(strings.ToLower(inst), "engtype_"); i >= 0 {
			kind = strings.ToLower(inst[i+len("engtype_"):])
		}
		if perType[key] == nil {
			perType[key] = map[string]float64{}
		}
		perType[key][kind] += v
	}
	out := map[string]float64{}
	for key, kinds := range perType {
		for _, v := range kinds {
			out[key] = max(out[key], v)
		}
	}
	return out
}
