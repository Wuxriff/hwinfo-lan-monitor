package app

import (
	"net"
	"net/http"
	"testing"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
)

func freePort(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func get(t *testing.T, addr string) int {
	t.Helper()
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET %s: %v", addr, err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestListenAgainOnSamePort(t *testing.T) {
	a := New(shm.ReadRaw)
	defer a.Close()
	addr := freePort(t)
	if err := a.Listen(addr); err != nil {
		t.Fatal(err)
	}
	if err := a.Listen(addr); err != nil {
		t.Fatalf("re-listen on the same port: %v", err)
	}
	if code := get(t, addr); code != http.StatusOK {
		t.Errorf("status %d", code)
	}
}

func TestListenOnBusyPortKeepsPrevious(t *testing.T) {
	a := New(shm.ReadRaw)
	defer a.Close()
	addr := freePort(t)
	if err := a.Listen(addr); err != nil {
		t.Fatal(err)
	}
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	if err := a.Listen(busy.Addr().String()); err == nil {
		t.Fatal("expected error on busy port")
	}
	if code := get(t, addr); code != http.StatusOK {
		t.Errorf("previous address not restored, status %d", code)
	}
}
