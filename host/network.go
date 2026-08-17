package host

import (
	"net"
	"os"
	"strings"
)

// NetworkInfo is the host's discovered names and non-loopback addresses.
type NetworkInfo struct {
	Hostnames []string
	IPs       []net.IP
}

type networkQuery interface {
	hostname() (string, error)
	lookupIP(string) ([]net.IP, error)
	lookupAddr(string) ([]string, error)
	interfaceAddrs() ([]net.Addr, error)
}

type networkSystem struct{}

func (networkSystem) hostname() (string, error)                { return os.Hostname() }
func (networkSystem) lookupIP(name string) ([]net.IP, error)   { return net.LookupIP(name) }
func (networkSystem) lookupAddr(addr string) ([]string, error) { return net.LookupAddr(addr) }
func (networkSystem) interfaceAddrs() ([]net.Addr, error)      { return net.InterfaceAddrs() }

// Network discovers the local host's names and non-loopback, non-link-local IP
// addresses. Discovery is best effort: unavailable DNS or interface data simply
// produces fewer values.
func Network() NetworkInfo { return discoverNetwork(networkSystem{}) }

// Hostnames returns the discovered hostnames, including the OS hostname, its
// short form, and reverse-DNS names for discovered addresses when available.
func Hostnames() []string { return Network().Hostnames }

// IPs returns discovered non-loopback, non-link-local interface addresses.
func IPs() []net.IP { return Network().IPs }

func discoverNetwork(query networkQuery) NetworkInfo {
	var out NetworkInfo
	name, err := query.hostname()
	if err == nil && name != "" {
		out.Hostnames = appendUniqueString(out.Hostnames, name)
		if short, _, ok := strings.Cut(name, "."); ok {
			out.Hostnames = appendUniqueString(out.Hostnames, short)
		}
		if ips, err := query.lookupIP(name); err == nil {
			out.IPs = appendIPs(out.IPs, ips)
		}
	}
	if addrs, err := query.interfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ip, ok := addrIP(addr); ok {
				out.IPs = appendIPs(out.IPs, []net.IP{ip})
			}
		}
	}
	for _, ip := range out.IPs {
		if names, err := query.lookupAddr(ip.String()); err == nil {
			for _, name := range names {
				out.Hostnames = appendUniqueString(out.Hostnames, strings.TrimSuffix(name, "."))
			}
		}
	}
	return out
}

func addrIP(addr net.Addr) (net.IP, bool) {
	ipNet, ok := addr.(*net.IPNet)
	if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() || ipNet.IP.IsLinkLocalMulticast() {
		return nil, false
	}
	return ipNet.IP, true
}

func appendIPs(current, values []net.IP) []net.IP {
	for _, ip := range values {
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			continue
		}
		found := false
		for _, existing := range current {
			found = found || existing.Equal(ip)
		}
		if !found {
			current = append(current, ip)
		}
	}
	return current
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
