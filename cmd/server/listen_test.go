package main

import "testing"

func TestListenAddr(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		want    string
		wantErr bool
	}{
		{"empty defaults", "", ":8080", false},
		{"valid", "9000", ":9000", false},
		{"min", "1", ":1", false},
		{"max", "65535", ":65535", false},
		{"zero", "0", "", true},
		{"too large", "65536", "", true},
		{"letters", "abc", "", true},
		{"trailing junk", "80a", "", true},
		{"negative", "-1", "", true},
		{"plus sign", "+80", "", true},
		{"spaces rejected", " 90 ", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listenAddr(tt.port)
			if (err != nil) != tt.wantErr {
				t.Fatalf("listenAddr(%q) err = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("listenAddr(%q) = %q, want %q", tt.port, got, tt.want)
			}
		})
	}
}
