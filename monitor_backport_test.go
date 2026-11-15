//go:build linux || windows || darwin

package tun

import (
	"errors"
	"github.com/sagernet/sing/common/control"
	"github.com/sagernet/sing/common/logger"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type backportFinder struct {
	control.InterfaceFinder
	update func() error
}

func (f backportFinder) Update() error { return f.update() }

// This test exercises shipped source without an overlay or platform route lookup.
func TestMonitorBackportSerializedFinder(t *testing.T) {
	var active, maximum atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	m := &defaultInterfaceMonitor{logger: logger.NOP()}
	m.interfaceFinder = backportFinder{update: func() error {
		n := active.Add(1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		once.Do(func() { close(entered); <-release })
		active.Add(-1)
		return errors.New("intentional finder failure")
	}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); m.postCheckUpdate() }()
	}
	<-entered
	close(release)
	wg.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("concurrent finder entries: %d", maximum.Load())
	}
}

// Real timer Reset, without an unowned AfterFunc callback; initialization and
// firing are exercised by the overlay with exhaustive timer ownership.
func TestMonitorBackportConcurrentReset(t *testing.T) {
	m := &defaultInterfaceMonitor{checkUpdateTimer: time.NewTimer(time.Hour)}
	defer m.checkUpdateTimer.Stop()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				m.delayCheckUpdate()
			}
		}()
	}
	wg.Wait()
}
