package main

import "testing"

func TestValidateRemote(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{name: "empty", addr: "", wantErr: true},
		{name: "host only", addr: "example.com", wantErr: true},
		{name: "port only", addr: ":443", wantErr: true},
		{name: "host empty port", addr: "example.com:", wantErr: true},
		{name: "host port", addr: "example.com:443", wantErr: false},
		{name: "ipv4 port", addr: "127.0.0.1:8443", wantErr: false},
		{name: "ipv6 port", addr: "[::1]:443", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRemote(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRemote(%q) err=%v wantErr=%v", tt.addr, err, tt.wantErr)
			}
		})
	}
}
