package port

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Allocator manages port reservations to eliminate race conditions.
type Allocator struct {
	reserved map[int]string // port -> service name
	mu       sync.Mutex
}

// NewAllocator creates a new port allocator.
func NewAllocator() *Allocator {
	return &Allocator{
		reserved: make(map[int]string),
	}
}

// Reserve claims a port for a service. If preferred is 0, picks a random
// available port. Returns the actual port assigned.
func (a *Allocator) Reserve(name string, preferred int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if preferred > 0 {
		if owner, taken := a.reserved[preferred]; taken {
			if owner == name {
				return preferred, nil
			}
			return 0, fmt.Errorf("port %d already reserved by %s", preferred, owner)
		}
		if !a.isPortFree(preferred) {
			return 0, fmt.Errorf("port %d is already in use", preferred)
		}
		a.reserved[preferred] = name
		return preferred, nil
	}

	for i := 0; i < 50; i++ {
		port := 20000 + rand.Intn(25000)
		if _, taken := a.reserved[port]; taken {
			continue
		}
		if !a.isPortFree(port) {
			continue
		}
		a.reserved[port] = name
		return port, nil
	}

	return 0, fmt.Errorf("no available port found for %s after 50 attempts", name)
}

// Release frees a port reservation for a service.
func (a *Allocator) Release(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for port, owner := range a.reserved {
		if owner == name {
			delete(a.reserved, port)
		}
	}
}

// IsAvailable checks if a port is currently free.
func (a *Allocator) IsAvailable(port int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, taken := a.reserved[port]; taken {
		return false
	}
	return a.isPortFree(port)
}

// FindListenerPIDs returns PIDs of processes listening on a port.
func (a *Allocator) FindListenerPIDs(port int) ([]int, error) {
	out, err := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port), "-sTCP:LISTEN").Output()
	if err != nil {
		return nil, nil
	}
	return parsePIDs(strings.TrimSpace(string(out))), nil
}

// StopListeners kills all processes listening on a port.
func (a *Allocator) StopListeners(port int) error {
	pids, err := a.FindListenerPIDs(port)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		_ = exec.Command("kill", strconv.Itoa(pid)).Run()
	}
	return nil
}

func (a *Allocator) isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func parsePIDs(output string) []int {
	if output == "" {
		return nil
	}
	var pids []int
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if pid, err := strconv.Atoi(line); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}
