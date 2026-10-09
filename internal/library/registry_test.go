package library

import "testing"

func TestRunningAppIDFromRegistry(t *testing.T) {
	doc := func(id string) []byte {
		return []byte(`"Registry"
{
	"HKCU"
	{
		"Software"
		{
			"Valve"
			{
				"Steam"
				{
					"language"		"english"
					"RunningAppID"		"` + id + `"
				}
			}
		}
	}
}`)
	}
	if got := runningAppIDFromRegistry(doc("620")); got != "620" {
		t.Errorf("running = %q", got)
	}
	for _, idle := range []string{"0", "", "abc"} {
		if got := runningAppIDFromRegistry(doc(idle)); got != "" {
			t.Errorf("%q should mean nothing is running, got %q", idle, got)
		}
	}
	if got := runningAppIDFromRegistry([]byte("not vdf {")); got != "" {
		t.Errorf("garbage = %q", got)
	}
}
