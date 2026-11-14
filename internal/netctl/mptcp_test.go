package netctl

import (
	"net"
	"testing"
)

// Защищает настройку godebug multipathtcp=0 в go.mod от случайного удаления.
func TestListenersDoNotUseMPTCP(t *testing.T) {
	lc := net.ListenConfig{}
	if lc.MultipathTCP() {
		t.Fatal("MPTCP enabled by default for listeners; check godebug multipathtcp=0 in go.mod")
	}
}
