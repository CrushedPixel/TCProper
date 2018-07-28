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
	Addr string

	connections chan Connection
	run         *atomic.Value
}

// NewServer creates a new TCP server that will listen for connections
// on the given address.
func NewServer(addr string) *Server {
	run := &atomic.Value{}
	run.Store(false)

	return &Server{
		Addr:        addr,
		connections: make(chan Connection, channelCapacity),
		run:         run,
	}
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

	// resolve the tcp address
	laddr, err := net.ResolveTCPAddr("tcp", s.Addr)
	if err != nil {
		return err
	}

	// start listening to incoming tcp connections
	l, err := net.ListenTCP("tcp", laddr)
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
