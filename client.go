package tcproper

import "net"

// Connect connects to a TCP server at the given address.
func Connect(network string, addr string) (Connection, error) {
	// resolve the tcp address
	raddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	// connect to the tcp server
	conn, err := net.DialTCP(network, nil, raddr)
	if err != nil {
		return nil, err
	}

	// create and return a Connection for the tcp connection
	return createConnection(conn), nil
}
