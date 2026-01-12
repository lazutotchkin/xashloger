package custom_middleware

import (
	"net"
	"net/http"
	"strings"
)

type IPFilter struct {
	allowedNets []*net.IPNet
}

func NewIPFilter(allowed []string) (*IPFilter, error) {
	var nets []*net.IPNet

	for _, a := range allowed {
		if strings.Contains(a, "/") {
			_, n, err := net.ParseCIDR(a)
			if err != nil {
				return nil, err
			}
			nets = append(nets, n)
			continue
		}

		ip := net.ParseIP(a)
		if ip == nil {
			return nil, errInvalidIP(a)
		}

		bits := 32
		if ip.To4() == nil {
			bits = 128
		}

		nets = append(nets, &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(bits, bits),
		})
	}

	return &IPFilter{allowedNets: nets}, nil
}

func (f *IPFilter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ip := clientIP(r)
		if ip == nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		for _, n := range f.allowedNets {
			if n.Contains(ip) {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

func clientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return net.ParseIP(strings.TrimSpace(parts[0]))
	}

	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return net.ParseIP(xr)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}

	return net.ParseIP(host)
}

func errInvalidIP(v string) error {
	return &net.ParseError{Type: "IP address", Text: v}
}
