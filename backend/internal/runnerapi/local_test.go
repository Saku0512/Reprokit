package runnerapi

import (
	"net"
	"testing"
)

func TestNewLocalServerBindsOnlyToIPv4Loopback(t *testing.T) {
	server, err := NewLocalServer(&fakeService{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Listener.Close()
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.Equal(net.ParseIP("127.0.0.1")) || len(server.SessionToken) < 32 {
		t.Fatalf("listener=%v token=%q", server.Listener.Addr(), server.SessionToken)
	}
}
