package userllm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// cgnat is 100.64.0.0/10, which netip doesn't classify as private.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// isPublicAddr reports whether ip is safe for the server to call on a user's
// behalf: not loopback, private, link-local (cloud metadata), CGNAT,
// multicast or unspecified.
func isPublicAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsInterfaceLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified() &&
		!cgnat.Contains(ip)
}

// newUserURLClient returns the http.Client for user-supplied base URLs (Ollama).
// The check runs in the dialer's Control hook, on the address actually being
// connected to, so DNS rebinding and redirects to internal hosts are caught
// too. allowPrivate turns the check off for self-hosted deployments whose
// Ollama runs on the private network.
func newUserURLClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if !isPublicAddr(ip) {
				return fmt.Errorf("refusing to connect to non-public address %s", ip)
			}
			return nil
		}
	}
	return &http.Client{
		Transport: &http.Transport{
			// No proxy: a proxy would make the dial check see the proxy, not the target.
			Proxy: nil,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, addr)
			},
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 5 * time.Minute,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}
