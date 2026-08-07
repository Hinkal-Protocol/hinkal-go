package dns

import (
	"context"
	"net"
	"runtime"
	"strings"

	"github.com/Hinkal-Protocol/hinkal-go/mobile/internal/constants"
)

// Android needed pure go dns resolver when using gomobile
// because gomobile does not have /etc/resolv.conf,
// ios shouldn't have that problem.
func init() {
	if runtime.GOOS == constants.AndroidOS {
		installResolver(constants.DefaultDNSServers)
	}
}

func SetServers(csv string) string {
	servers := parseServers(csv)
	installResolver(servers)
	return strings.Join(servers, ",")
}

func parseServers(csv string) []string {
	servers := make([]string, 0, 4)
	for _, part := range strings.Split(csv, ",") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			servers = append(servers, net.JoinHostPort(s, constants.DNSPort))
			continue
		}
		if host, port, err := net.SplitHostPort(s); err == nil && net.ParseIP(host) != nil && port != "" {
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 {
		return constants.DefaultDNSServers
	}
	return servers
}

func installResolver(servers []string) {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: constants.DNSDialTimeout}
			var lastErr error
			for _, server := range servers {
				conn, err := d.DialContext(ctx, network, server)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
}
