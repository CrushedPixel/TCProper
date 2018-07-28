package tcproper

import "net"

// Connect connects to a TCP server at the given address.
func Connect(addr string) (Connection, error) {
	// resolve the tcp address
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}

	// connect to the tcp server
	conn, err := net.DialTCP("tcp", nil, raddr)
	if err != nil {
		return nil, err
	}

	// create and return a Connection for the tcp connection
	return createConnection(conn), nil
}
