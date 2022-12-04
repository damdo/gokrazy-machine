package ports

import (
	"errors"
	"net"
)

var ErrFailedAssertionTCPAddr = errors.New("failed assertion for *net.TCPAddr")

// credits: github.com/phayes/freeport

// GetFreePorts asks the kernel for free open ports that are ready to use.
func GetFreePorts(count int) ([]int, error) {
	var ports []int
	for i := 0; i < count; i++ {
		addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}

		l, err := net.ListenTCP("tcp", addr)
		if err != nil {
			return nil, err
		}
		defer l.Close()

		a, ok := l.Addr().(*net.TCPAddr)
		if !ok {
			return nil, ErrFailedAssertionTCPAddr
		}

		ports = append(ports, a.Port)
	}
	return ports, nil
}
