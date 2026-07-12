package webrtcx

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
)

// DCConn adapts a WebRTC DataChannel to net.Conn-ish ReadWriteCloser for COE framing.
type DCConn struct {
	dc     *webrtc.DataChannel
	pc     *webrtc.PeerConnection
	incoming chan []byte
	buf    []byte
	mu     sync.Mutex
	closed bool
	rmu    sync.Mutex
	rcond  *sync.Cond
}

// NewDCConn wraps an open data channel.
func NewDCConn(pc *webrtc.PeerConnection, dc *webrtc.DataChannel) *DCConn {
	c := &DCConn{
		dc:       dc,
		pc:       pc,
		incoming: make(chan []byte, 64),
	}
	c.rcond = sync.NewCond(&c.rmu)
	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()
		// copy
		b := append([]byte(nil), msg.Data...)
		select {
		case c.incoming <- b:
		default:
			// drop if flooded
		}
		c.rcond.Broadcast()
	})
	dc.OnClose(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		c.rcond.Broadcast()
	})
	return c
}

func (c *DCConn) Read(p []byte) (int, error) {
	for {
		if len(c.buf) > 0 {
			n := copy(p, c.buf)
			c.buf = c.buf[n:]
			return n, nil
		}
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return 0, io.EOF
		}
		select {
		case b, ok := <-c.incoming:
			if !ok {
				return 0, io.EOF
			}
			c.buf = b
		case <-time.After(30 * time.Second):
			// continue loop / allow deadline later
			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return 0, io.EOF
			}
			c.mu.Unlock()
		}
	}
}

func (c *DCConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, fmt.Errorf("closed")
	}
	if err := c.dc.Send(append([]byte(nil), p...)); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *DCConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	_ = c.dc.Close()
	return c.pc.Close()
}

func (c *DCConn) LocalAddr() net.Addr                { return addr("webrtc-local") }
func (c *DCConn) RemoteAddr() net.Addr               { return addr("webrtc-remote") }
func (c *DCConn) SetDeadline(t time.Time) error      { return nil }
func (c *DCConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *DCConn) SetWriteDeadline(t time.Time) error { return nil }

type addr string

func (a addr) Network() string { return "webrtc" }
func (a addr) String() string  { return string(a) }
