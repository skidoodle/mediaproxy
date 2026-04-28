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
func isSafeFetchableHost(host string) bool {
	hostname := host
	if strings.Contains(host, ":") {
		h, _, err := net.SplitHostPort(host)
		if err == nil {
			hostname = h
		}
	}

	hostname = strings.TrimSuffix(hostname, ".")
	hostnameLower := strings.ToLower(hostname)

	// Block explicitly localhost and local domains
	if hostnameLower == "localhost" || strings.HasSuffix(hostnameLower, ".local") || strings.HasSuffix(hostnameLower, ".internal") {
		return false
	}

	// If it parses as an IP address, we strictly block loopback, private, and unspecified ranges.
	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			return false
		}
		return true // Valid public IP
	}

	// Must have at least one dot to be a valid public TLD/Domain
	if !strings.Contains(hostname, ".") {
		return false
	}

	return true
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
