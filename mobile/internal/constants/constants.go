package constants

import "time"

var DefaultDNSServers = []string{"8.8.8.8:53", "1.1.1.1:53"}

const (
	DNSPort        = "53"
	DNSDialTimeout = 5 * time.Second

	AndroidOS = "android"
)
