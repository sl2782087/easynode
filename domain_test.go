package main

import (
	"net/netip"
	"testing"
)

func TestDomainDNSMustPointToPanel(t *testing.T) {
	v4 := netip.MustParseAddr("203.0.113.8")
	v6 := netip.MustParseAddr("2001:db8::8")
	allowed := map[netip.Addr]bool{v4: true, v6: true, netip.MustParseAddr("127.0.0.1"): true}
	for _, tc := range []struct {
		records []netip.Addr
		valid   bool
	}{
		{nil, false}, {[]netip.Addr{v4, v6}, true},
		{[]netip.Addr{netip.MustParseAddr("::ffff:203.0.113.8")}, true},
		{[]netip.Addr{v4, netip.MustParseAddr("2001:db8::9")}, false},
		{[]netip.Addr{netip.MustParseAddr("127.0.0.1")}, false},
	} {
		if pointsHere(tc.records, allowed) != tc.valid {
			t.Errorf("unexpected DNS decision for %v", tc.records)
		}
	}
}
