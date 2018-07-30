package tcproper

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

const (
	channelCapacity          = 10
	connectionAcceptInterval = 5 * time.Second
)

var ErrServerAlreadyRun = errors.New("a server may only be run once")

type Server struct {
	laddr   *net.TCPAddr
	network string

	connections chan Connection
	run         *atomic.Value
}

// NewServer creates a new TCP server that will listen for connections on the given address.
// The network must be a TCP network name, e.g. "tcp", "tcp4" or "tcp6".
func NewServer(network string, addr string) (*Server, error) {
	run := &atomic.Value{}
	run.Store(false)

	// resolve the tcp address
	laddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	return &Server{
		laddr:   laddr,
		network: network,

		connections: make(chan Connection, channelCapacity),
		run:         run,
	}, nil
}

// Connections returns a channel receiving incoming connections.
// The channel is closed when the Server stops listening.
func (s *Server) Connections() <-chan Connection {
	return s.connections
}

// Run listens for connections to the server address until ctx is Done.
// This is a blocking operation.
func (s *Server) Run(ctx context.Context) error {
	if s.run.Load().(bool) {
		return ErrServerAlreadyRun
	}
	s.run.Store(true)

	// close the connections channel when Run returns
	defer close(s.connections)

	// start listening to incoming tcp connections
	l, err := net.ListenTCP(s.network, s.laddr)
	if err != nil {
		return err
	}
	defer l.Close()

	// loop accepting connections
	for {
		// check whether the context is done
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// set a deadline for the accept call, so we can frequently
		// check whether the context is done
		if err := l.SetDeadline(time.Now().Add(connectionAcceptInterval)); err != nil {
			return err
		}

		conn, err := l.AcceptTCP()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				// we didn't receive a new connection within the deadline
				continue
			}

			// an error occurred when accepting the connection
			return err
		}

		// we have a new incoming tcp connection.
		// Create a Connection for it and feed it
		// into the connections channel without blocking
		// the accept loop.
		go func() {
			c := createConnection(conn)
			s.connections <- c
		}()
	}

	return nil
}
