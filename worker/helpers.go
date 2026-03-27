package worker

import "fmt"

func buildAuthority(host string, port int) string {
	if port == 80 || port == 443 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}
