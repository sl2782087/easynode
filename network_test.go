package main

import "testing"

func TestParseGeo(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		ipv4, ok bool
	}{
		{"203.0.113.12\r\n美国 洛杉矶\r\nAS174\r\nCogent\r\n", true, true},
		{"2001:db8::1\n日本\nAS1\nExample", false, true},
		{"203.0.113.12\n美国\nAS1\nExample", false, false},
		{"<html>rate limited</html>", true, false},
	} {
		if _, ok := parseGeo([]byte(tc.raw), tc.ipv4); ok != tc.ok {
			t.Errorf("unexpected result %v for %q", ok, tc.raw)
		}
	}
}
