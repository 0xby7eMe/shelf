package sysmon

import "testing"

func TestEngineUsage(t *testing.T) {
	got := engineUsage(map[string]float64{
		"pid_10_luid_0x00000000_0x0000D3A5_phys_0_eng_0_engtype_3D":          30,
		"pid_11_luid_0x00000000_0x0000D3A5_phys_0_eng_0_engtype_3D":          25,
		"pid_11_luid_0x00000000_0x0000D3A5_phys_0_eng_3_engtype_VideoDecode": 40,
		"pid_12_luid_0x00000000_0x0000BEEF_phys_0_eng_1_engtype_Copy":        5,
		"not a gpu instance": 99,
	})
	if v := got["luid_0x00000000_0x0000d3a5"]; v != 55 {
		t.Errorf("busiest engine of the first card = %v, want the 3D sum 55", v)
	}
	if v := got["luid_0x00000000_0x0000beef"]; v != 5 {
		t.Errorf("second card = %v", v)
	}
	if len(got) != 2 {
		t.Errorf("cards: %v", got)
	}
	if adapterOf("luid_0x00000000_0x0000D3A5_phys_0") != "luid_0x00000000_0x0000d3a5" {
		t.Error("memory counter instance not matched to its card")
	}
}
