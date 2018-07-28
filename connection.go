package tcproper

import (
	"encoding/binary"
	"net"
	"time"
)

const bufSize = 1024

type Connection interface {
	RemoteAddr() net.Addr

	// Read returns a channel receiving
	// all messages received by this Connection.
	// The channel is closed when the Connection is closed.
	Read() <-chan []byte

	// Write sends a message to this Connection.
	// If the connection is already closed, Write does nothing.
	Write([]byte)

	// Closed returns a channel that's closed
	// when the connection has been closed.
	// This can be used similarly to Context.Done().
	Closed() <-chan struct{}

	// If Closed is not yet closed, Err returns nil.
	// Otherwise, it returns the reason why the connection was closed.
	Err() error

	// Close closes the connection, setting its error to err.
	// If the Connection is already closed, Close does nothing.
	Close(err error)
}

// conn implements Connection.
type conn struct {
	conn *net.TCPConn

	// channel that, when closed,
	// indicates that the connection has been closed.
	closed chan struct{}

	read  chan []byte
	write chan []byte

	err error
}

func createConnection(tcpConn *net.TCPConn) Connection {
	c := &conn{
		conn:   tcpConn,
		closed: make(chan struct{}),
		read:   make(chan []byte, channelCapacity),
		write:  make(chan []byte, channelCapacity),
	}

	go c.readLoop()
	go c.writeLoop()

	return c
}

func (c *conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *conn) Read() <-chan []byte {
	return c.read
}

func (c *conn) Write(msg []byte) {
	select {
	case <-c.Closed():
		return
	default:
	}

	c.write <- msg
}

func (c *conn) Closed() <-chan struct{} {
	return c.closed
}

func (c *conn) Close(err error) {
	select {
	case <-c.Closed():
		return
	default:
	}

	c.err = err
	close(c.closed)
}

func (c *conn) Err() error {
	return c.err
}

func (c *conn) readLoop() {
	// close read channel once connection is closed
	defer close(c.read)

	// create read buffer
	buf := make([]byte, bufSize)

	var msgLen uint64
	var msgLenBuf []byte
	var msg []byte

	for {
		select {
		case <-c.Closed():
			return
		default:
		}

		if err := c.conn.SetReadDeadline(time.Now().Add(connectionAcceptInterval)); err != nil {
			c.Close(err)
			return
		}

		n, err := c.conn.Read(buf)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				// we didn't receive a message within the deadline
				continue
			}

			c.Close(err)
			return
		}

		i := 0 // buf read index
		for {
			if msgLen == 0 {
				// we need to parse the message length
				msgLenBuf = append(msgLenBuf, buf[i])

				if len(msgLenBuf) == 8 {
					// we have gathered enough bytes to parse
					// the message length
					msgLen = binary.BigEndian.Uint64(msgLenBuf)
					msgLenBuf = nil
				}
			} else {
				msg = append(msg, buf[i])
				msgLen--

				if msgLen == 0 {
					// the whole message was read -
					// write it into the read channel
					select {
					case c.read <- msg:
					case <-c.Closed():
						return
					}

					msg = nil
				}
			}

			i++

			if i >= n {
				// we reached the end of the received data
				break
			}
		}
	}
}

func (c *conn) writeLoop() {
	// close write channel once connection is closed
	defer close(c.write)

	for {
		select {
		case <-c.Closed():
			return
		case msg := <-c.write:
			// prepend the length of the message as an uint64
			lenBytes := make([]byte, 8)
			binary.BigEndian.PutUint64(lenBytes, uint64(len(msg)))
			msg = append(lenBytes, msg...)
			if _, err := c.conn.Write(msg); err != nil {
				c.Close(err)
				return
			}
		}
	}
}
