package api

import "testing"

func TestVehicleReturnPath(t *testing.T) {
	for _, path := range []string{"", "https://example.org", "//example.org", "/%2fexample.org", "/%5cexample.org", "/path\r\nLocation: elsewhere"} {
		if got := vehicleReturnPath(path, "/vehicles"); got != "/vehicles" {
			t.Fatalf("unsafe return %q: %q", path, got)
		}
	}
	if got := vehicleReturnPath("/object/object?tab=photos", "/vehicles"); got != "/object/object?tab=photos" {
		t.Fatal(got)
	}
}
