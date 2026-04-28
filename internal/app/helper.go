package app

import (
	"bytes"
	"fmt"
	"image/gif"
	"net"
	"strings"

	"github.com/h2non/bimg"
)

// isAllowedDomain checks if the provided host matches any of the domains
// in the allowed whitelist. It supports exact matches and subdomains.
// Returns true if the allowedDomains list is empty (allowing all domains).
func isAllowedDomain(host string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	for _, domain := range allowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

// isSafeFetchableHost validates that the target host is a valid, external domain or IP.
// It explicitly blocks internal/private IP ranges, localhost, and malformed strings
// to prevent Server-Side Request Forgery (SSRF) and useless fetch attempts.
func (app *App) isSafeFetchableHost(host string) bool {
	hostname := host
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			hostname = h
		}
	}

	hostname = strings.TrimSuffix(hostname, ".")
	hostnameLower := strings.ToLower(hostname)

	// Block explicitly localhost and local domains (unless explicitly allowed)
	if !app.Config.AllowPrivateIPs {
		if hostnameLower == "localhost" || strings.HasSuffix(hostnameLower, ".local") || strings.HasSuffix(hostnameLower, ".internal") || strings.HasSuffix(hostnameLower, ".lan") {
			return false
		}
	}

	// If it parses as an IP address, we check if it is a bogon/private IP.
	if ip := net.ParseIP(hostname); ip != nil {
		return app.isSafeIP(ip)
	}

	// Must have at least one dot to be a valid public TLD/Domain
	if !strings.Contains(hostname, ".") {
		return false
	}

	return true
}

var (
	bogonIPv4 []*net.IPNet
	bogonIPv6 []*net.IPNet
)

func init() {
	// IPv4 Bogon/Private Ranges
	ipv4CIDRs := []string{
		"0.0.0.0/8",          // "This" network
		"10.0.0.0/8",         // Private-use
		"100.64.0.0/10",      // Carrier-grade NAT
		"127.0.0.0/8",        // Loopback
		"169.254.0.0/16",     // Link-local
		"172.16.0.0/12",      // Private-use
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1
		"192.88.99.0/24",     // 6to4 Relay
		"192.168.0.0/16",     // Private-use
		"198.18.0.0/15",      // Benchmarking
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // Multicast
		"240.0.0.0/4",        // Reserved
		"255.255.255.255/32", // Limited broadcast
	}

	// IPv6 Bogon/Private Ranges
	ipv6CIDRs := []string{
		"::/128",                // Unspecified
		"::1/128",               // Loopback
		"::/96",                 // IPv4-compatible (deprecated)
		"100::/64",              // Black hole
		"2001:db8::/32",         // Documentation
		"2001:10::/28",          // ORCHIDv2
		"2001:20::/28",          // ORCHIDv2
		"fc00::/7",              // ULA
		"fe80::/10",             // Link-local
		"ff00::/8",              // Multicast
		"2002::/24",             // 6to4 bogon (0.0.0.0/8)
		"2002:a00::/24",         // 6to4 bogon (10.0.0.0/8)
		"2002:7f00::/24",        // 6to4 bogon (127.0.0.0/8)
		"2002:a9fe::/32",        // 6to4 bogon (169.254.0.0/16)
		"2002:ac10::/28",        // 6to4 bogon (172.16.0.0/12)
		"2002:c000::/40",        // 6to4 bogon (192.0.0.0/24)
		"2002:c000:200::/40",    // 6to4 bogon (192.0.2.0/24)
		"2002:c0a8::/32",        // 6to4 bogon (192.168.0.0/16)
		"2002:c612::/31",        // 6to4 bogon (198.18.0.0/15)
		"2002:c633:6400::/40",   // 6to4 bogon (198.51.100.0/24)
		"2002:cb00:7100::/40",   // 6to4 bogon (203.0.113.0/24)
		"2002:e000::/20",        // 6to4 bogon (224.0.0.0/4)
		"2002:f000::/20",        // 6to4 bogon (240.0.0.0/4)
		"2002:ffff:ffff::/48",   // 6to4 bogon (255.255.255.255/32)
		"2001::/40",             // Teredo bogon (0.0.0.0/8)
		"2001:0:a00::/40",       // Teredo bogon (10.0.0.0/8)
		"2001:0:7f00::/40",      // Teredo bogon (127.0.0.0/8)
		"2001:0:a9fe::/48",      // Teredo bogon (169.254.0.0/16)
		"2001:0:ac10::/44",      // Teredo bogon (172.16.0.0/12)
		"2001:0:c000::/56",      // Teredo bogon (192.0.0.0/24)
		"2001:0:c000:200::/56",  // Teredo bogon (192.0.2.0/24)
		"2001:0:c0a8::/48",      // Teredo bogon (192.168.0.0/16)
		"2001:0:c612::/47",      // Teredo bogon (198.18.0.0/15)
		"2001:0:c633:6400::/56", // Teredo bogon (198.51.100.0/24)
		"2001:0:cb00:7100::/56", // Teredo bogon (203.0.113.0/24)
		"2001:0:e000::/36",      // Teredo bogon (224.0.0.0/4)
		"2001:0:f000::/36",      // Teredo bogon (240.0.0.0/4)
		"2001:0:ffff:ffff::/64", // Teredo bogon (255.255.255.255/32)
	}

	for _, cidr := range ipv4CIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			bogonIPv4 = append(bogonIPv4, n)
		}
	}

	for _, cidr := range ipv6CIDRs {
		_, n, err := net.ParseCIDR(cidr)
		if err == nil {
			bogonIPv6 = append(bogonIPv6, n)
		}
	}
}

// isSafeIP checks if an IP address is safe to fetch based on app configuration.
func (app *App) isSafeIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if app.Config.AllowPrivateIPs {
		return true
	}

	return !isBogonIP(ip)
}

// isBogonIP checks if an IP address is a known bogon or private address.
func isBogonIP(ip net.IP) bool {
	// Standard library checks (fast path)
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// If it is an IPv4 address (even if mapped), check IPv4 bogon ranges.
	if ip4 := ip.To4(); ip4 != nil {
		for _, network := range bogonIPv4 {
			if network.Contains(ip4) {
				return true
			}
		}
		return false
	}

	// Otherwise, check IPv6 bogon ranges.
	for _, network := range bogonIPv6 {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// isGif decodes the provided byte slice to determine if it represents an animated GIF.
// It returns true if the GIF contains more than one frame.
func isGif(data []byte) (bool, error) {
	r := bytes.NewReader(data)
	g, err := gif.DecodeAll(r)
	if err != nil {
		return false, err
	}
	return len(g.Image) > 1, nil
}

// optimizeMedia converts a static image byte slice to the WebP format,
// strips its metadata, and applies the specified quality setting to reduce file size.
func optimizeMedia(data []byte, quality int) ([]byte, error) {
	processed, err := bimg.NewImage(data).Process(bimg.Options{
		Type:          bimg.WEBP,
		Quality:       quality,
		StripMetadata: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to optimize image: %w", err)
	}
	return processed, nil
}
