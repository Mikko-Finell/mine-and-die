package websocket

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

const (
	TextMessage                  = 1
	BinaryMessage                = 2
	CloseMessage                 = 8
	PingMessage                  = 9
	PongMessage                  = 10
	CloseNormalClosure           = 1000
	CloseGoingAway               = 1001
	CloseProtocolError           = 1002
	CloseUnsupportedData         = 1003
	CloseNoStatusReceived        = 1005
	CloseAbnormalClosure         = 1006
	CloseInvalidFramePayloadData = 1007
	ClosePolicyViolation         = 1008
	CloseMessageTooBig           = 1009
	CloseMandatoryExtension      = 1010
	CloseInternalServerErr       = 1011
	CloseServiceRestart          = 1012
	CloseTryAgainLater           = 1013
	CloseTLSHandshake            = 1015
)

type Conn struct {
	mu     sync.Mutex
	closed bool
}

func (c *Conn) WriteMessage(messageType int, data []byte) error {
	if c == nil {
		return errors.New("websocket: nil connection")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("websocket: write on closed connection")
	}
	return nil
}

func (c *Conn) ReadMessage() (int, []byte, error) {
	if c == nil {
		return 0, nil, errors.New("websocket: nil connection")
	}
	return 0, nil, errors.New("websocket: read not supported in stub")
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	if c == nil {
		return errors.New("websocket: nil connection")
	}
	return nil
}

func (c *Conn) Close() error {
	if c == nil {
		return errors.New("websocket: nil connection")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

type Upgrader struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*Conn, error) {
	if u != nil && u.CheckOrigin != nil {
		if !u.CheckOrigin(r) {
			return nil, errors.New("websocket: origin not allowed")
		}
	}
	return &Conn{}, nil
}

func FormatCloseMessage(closeCode int, text string) []byte {
	return []byte(text)
}

var ErrCloseSent = errors.New("websocket: close sent")
