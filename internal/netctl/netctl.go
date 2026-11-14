// Package netctl предоставляет глобальный Control-хук для сокетов (VpnService.protect).
package netctl

import (
	"net"
	"sync/atomic"
	"syscall"
)

type ControlFunc func(network, address string, c syscall.RawConn) error

var control atomic.Pointer[ControlFunc]

// SetControl регистрирует функцию защиты сокетов хоста (nil - no-op).
func SetControl(fn ControlFunc) {
	if fn == nil {
		control.Store(nil)
		return
	}
	control.Store(&fn)
}

// Apply вызывается из net.Dialer и net.ListenConfig для защиты создаваемых сокетов.
func Apply(network, address string, c syscall.RawConn) error {
	if fn := control.Load(); fn != nil {
		return (*fn)(network, address, c)
	}
	return nil
}

// NoMPTCP отключает MPTCP явно, в том числе в сборках gomobile,
// где настройка godebug из go.mod может не применяться.
func NoMPTCP(cfg *net.ListenConfig) *net.ListenConfig {
	if cfg == nil {
		cfg = &net.ListenConfig{}
	}
	cfg.SetMultipathTCP(false)
	return cfg
}
