// Package app runs the SHM poller and the Dashboard web server, independent of any UI.
package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wuxriff/hwinfo-lan-monitor/internal/server"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/shm"
	"github.com/Wuxriff/hwinfo-lan-monitor/internal/source"
	"github.com/Wuxriff/hwinfo-lan-monitor/web"
)

type App struct {
	Source  *source.Source
	handler http.Handler

	mu         sync.Mutex
	srv        *http.Server
	ln         net.Listener
	addr       *net.TCPAddr
	listenAddr string
}

func New() *App {
	src := source.New(shm.ReadRaw)
	return &App{Source: src, handler: server.New(src, web.FS())}
}

// Run polls HWiNFO until ctx is done.
func (a *App) Run(ctx context.Context) { a.Source.Run(ctx) }

// Listen (re)starts the web server on addr. The old server is closed first because
// the new address usually reuses its port; if addr cannot be bound, the previous
// address is restored and the error returned. Open SSE streams reconnect by themselves.
func (a *App) Listen(addr string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if a.listenAddr != "" {
			if prev, perr := net.Listen("tcp", a.listenAddr); perr == nil {
				a.serve(prev)
			}
		}
		return err
	}
	a.listenAddr = addr
	a.serve(ln)
	return nil
}

func (a *App) serve(ln net.Listener) {
	srv := &http.Server{Handler: a.handler, ReadHeaderTimeout: 5 * time.Second}
	a.srv, a.ln, a.addr = srv, ln, ln.Addr().(*net.TCPAddr)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ln.Close()
		}
	}()
}

func (a *App) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopLocked()
}

// stopLocked closes the listener itself too: Server.Close only closes listeners
// that Serve has already registered, which races with a just-started goroutine.
func (a *App) stopLocked() {
	if a.srv != nil {
		a.srv.Close()
		a.ln.Close()
		a.srv, a.ln = nil, nil
	}
}

// URLs lists Dashboard addresses: LAN IPv4 addresses first, then localhost.
func (a *App) URLs() []string {
	a.mu.Lock()
	ad := a.addr
	a.mu.Unlock()
	if ad == nil {
		return nil
	}
	port := strconv.Itoa(ad.Port)
	if !ad.IP.IsUnspecified() {
		return []string{"http://" + net.JoinHostPort(ad.IP.String(), port)}
	}
	var out []string
	for _, ip := range lanIPs() {
		out = append(out, "http://"+ip+":"+port)
	}
	return append(out, "http://localhost:"+port)
}

// lanIPs lists IPv4 addresses of active interfaces, physical adapters before
// virtual ones (Hyper-V/WSL/Docker/VM switches) so the first entry suits a phone.
func lanIPs() []string {
	var real, virtual []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			if isVirtual(ifc.Name) {
				virtual = append(virtual, ipn.IP.String())
			} else {
				real = append(real, ipn.IP.String())
			}
		}
	}
	return append(real, virtual...)
}

func isVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, v := range []string{"vethernet", "wsl", "docker", "vmware", "virtualbox", "hyper-v", "vpn"} {
		if strings.Contains(n, v) {
			return true
		}
	}
	return false
}
