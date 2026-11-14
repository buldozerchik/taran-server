package netctl

import (
	"net"
	"syscall"
	"testing"
)

func TestNoMPTCPForcesMPTCPOff(t *testing.T) {
	cfg := &net.ListenConfig{}
	cfg.SetMultipathTCP(true)
	if !cfg.MultipathTCP() {
		t.Skip("MPTCP cannot be enabled explicitly on this platform")
	}
	if NoMPTCP(cfg).MultipathTCP() {
		t.Fatal("NoMPTCP left MPTCP enabled")
	}
}

func TestNoMPTCPAcceptsNil(t *testing.T) {
	cfg := NoMPTCP(nil)
	if cfg == nil || cfg.MultipathTCP() {
		t.Fatal("NoMPTCP(nil) must return a config with MPTCP disabled")
	}
}

func TestNoMPTCPPreservesConfig(t *testing.T) {
	called := false
	cfg := &net.ListenConfig{KeepAlive: -1}
	cfg.Control = func(_, _ string, _ syscall.RawConn) error {
		called = true
		return nil
	}
	got := NoMPTCP(cfg)
	if got != cfg || got.KeepAlive != -1 || got.Control == nil {
		t.Fatal("NoMPTCP replaced the listener configuration")
	}
	if err := got.Control("tcp", "127.0.0.1:0", nil); err != nil || !called {
		t.Fatal("NoMPTCP did not preserve the socket control hook")
	}
}
