package main

import "fmt"

const defaultListenAddr = ":8080"

// listenAddr turns the PORT env value into a listen address. Empty means the
// default; otherwise it must be a plain decimal integer in 1..65535 (no sign,
// no surrounding whitespace).
func listenAddr(port string) (string, error) {
	if port == "" {
		return defaultListenAddr, nil
	}
	n := 0
	for _, c := range port {
		if c < '0' || c > '9' || len(port) > 5 {
			return "", fmt.Errorf("invalid PORT %q: must be an integer between 1 and 65535", port)
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid PORT %q: must be an integer between 1 and 65535", port)
	}
	return fmt.Sprintf(":%d", n), nil
}
