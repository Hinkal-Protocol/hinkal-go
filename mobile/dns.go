package mobile

import (
	dns "github.com/Hinkal-Protocol/hinkal-go/mobile/internal/functions/dns"
)

func SetDNSServers(csv string) string {
	return dns.SetServers(csv)
}
