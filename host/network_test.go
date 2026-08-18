package host

import (
	"net"
	"testing"
)

type fakeNetworkQuery struct {
	host    string
	ips     []net.IP
	addrs   []net.Addr
	reverse map[string][]string
}

func (f fakeNetworkQuery) hostname() (string, error)                { return f.host, nil }
func (f fakeNetworkQuery) lookupIP(string) ([]net.IP, error)        { return f.ips, nil }
func (f fakeNetworkQuery) lookupAddr(addr string) ([]string, error) { return f.reverse[addr], nil }
func (f fakeNetworkQuery) interfaceAddrs() ([]net.Addr, error)      { return f.addrs, nil }

func TestDiscoverNetworkFiltersAndDeduplicates(t *testing.T) {
	q := fakeNetworkQuery{
		host:    "studio.local",
		ips:     []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.10")},
		addrs:   []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.10")}, &net.IPNet{IP: net.ParseIP("fe80::1")}},
		reverse: map[string][]string{"192.168.1.10": {"studio.example."}},
	}
	got := discoverNetwork(q)
	if len(got.IPs) != 1 || got.IPs[0].String() != "192.168.1.10" {
		t.Fatalf("ips=%v", got.IPs)
	}
	want := []string{"studio.local", "studio", "studio.example"}
	if len(got.Hostnames) != len(want) {
		t.Fatalf("hostnames=%v", got.Hostnames)
	}
	for i := range want {
		if got.Hostnames[i] != want[i] {
			t.Fatalf("hostnames=%v", got.Hostnames)
		}
	}
}
