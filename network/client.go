package network

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/automoto/doomerang-mp/shared/messages"
	"github.com/coder/websocket"
	"github.com/leap-fish/necs/esync"
	"github.com/leap-fish/necs/router"
	"github.com/leap-fish/necs/transports"
)

type ClientState int

const (
	StateDisconnected ClientState = iota
	StateConnecting
	StateConnected
	StateJoinedGame
	StateError
)

// Client manages a WebSocket connection to the game server.
// All shared fields are protected by mu (router callbacks run on necs goroutines).
type Client struct {
	mu sync.RWMutex

	state          ClientState
	lastError      error
	networkID      esync.NetworkId
	reconnectToken string
	serverName     string
	tickRate       int
	level          string
	conn           *websocket.Conn

	snapshotCh chan esync.WorldSnapshot // size-1 buffered; latest wins

	chargeCh chan messages.BoomerangChargeEvent
	throwCh  chan messages.BoomerangThrowEvent
	catchCh  chan messages.BoomerangCatchEvent
	hitCh    chan messages.BoomerangHitEvent
}

func NewClient() *Client {
	return &Client{
		state:      StateDisconnected,
		snapshotCh: make(chan esync.WorldSnapshot, 1),
		chargeCh:   make(chan messages.BoomerangChargeEvent, 4),
		throwCh:    make(chan messages.BoomerangThrowEvent, 4),
		catchCh:    make(chan messages.BoomerangCatchEvent, 4),
		hitCh:      make(chan messages.BoomerangHitEvent, 4),
	}
}

// Connect dials the server in a background goroutine and initiates the join handshake.
func (c *Client) Connect(address, version, playerName, level string) {
	c.mu.Lock()
	c.state = StateConnecting
	c.lastError = nil
	c.mu.Unlock()

	router.OnConnect(func(_ *router.NetworkClient) {
		log.Println("[client] connected to server")
		c.mu.Lock()
		c.state = StateConnected
		c.mu.Unlock()

		payload, err := router.Serialize(messages.JoinRequest{
			Version:    version,
			PlayerName: playerName,
			Level:      level,
		})
		if err != nil {
			c.setError(fmt.Errorf("failed to serialize join request: %w", err))
			return
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn != nil {
			if err := conn.Write(context.Background(), websocket.MessageBinary, payload); err != nil {
				c.setError(fmt.Errorf("failed to send join request: %w", err))
			}
		}
	})

	router.On(func(_ *router.NetworkClient, msg messages.JoinAccepted) {
		log.Printf("[client] join accepted: networkID=%d server=%s tickRate=%d",
			msg.NetworkID, msg.ServerName, msg.TickRate)
		c.mu.Lock()
		c.networkID = msg.NetworkID
		c.reconnectToken = msg.ReconnectToken
		c.serverName = msg.ServerName
		c.tickRate = msg.TickRate
		c.level = msg.Level
		c.state = StateJoinedGame
		c.mu.Unlock()
	})

	router.On(func(_ *router.NetworkClient, msg messages.JoinRejected) {
		log.Printf("[client] join rejected: %s", msg.Reason)
		c.setError(fmt.Errorf("join rejected: %s", msg.Reason))
	})

	router.On(func(_ *router.NetworkClient, snapshot esync.WorldSnapshot) {
		select { // drain stale, push latest
		case <-c.snapshotCh:
		default:
		}
		c.snapshotCh <- snapshot
	})

	router.On(func(_ *router.NetworkClient, evt messages.BoomerangChargeEvent) {
		select {
		case c.chargeCh <- evt:
		default:
		}
	})

	router.On(func(_ *router.NetworkClient, evt messages.BoomerangThrowEvent) {
		select {
		case c.throwCh <- evt:
		default:
		}
	})

	router.On(func(_ *router.NetworkClient, evt messages.BoomerangCatchEvent) {
		select {
		case c.catchCh <- evt:
		default:
		}
	})

	router.On(func(_ *router.NetworkClient, evt messages.BoomerangHitEvent) {
		select {
		case c.hitCh <- evt:
		default:
		}
	})

	router.OnDisconnect(func(_ *router.NetworkClient, err error) {
		log.Printf("[client] disconnected: %v", err)
		c.mu.Lock()
		if c.state != StateError {
			c.state = StateDisconnected
		}
		c.conn = nil
		c.mu.Unlock()
	})

	router.OnError(func(_ *router.NetworkClient, err error) {
		log.Printf("[client] error: %v", err)
	})

	go func() {
		transport := transports.NewWsClientTransport("ws://" + address)
		err := transport.Start(func(conn *websocket.Conn) {
			c.mu.Lock()
			c.conn = conn
			c.mu.Unlock()
		})
		if err != nil {
			c.setError(fmt.Errorf("connection failed: %w", err))
		}
	}()
}

func (c *Client) Disconnect() {
	c.mu.Lock()
	conn := c.conn
	c.state = StateDisconnected
	c.conn = nil
	c.mu.Unlock()

	if conn != nil {
		_ = conn.CloseNow()
	}

	router.ResetRouter()
}

func (c *Client) State() ClientState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) LastError() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastError
}

func (c *Client) NetworkID() esync.NetworkId {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.networkID
}

func (c *Client) Level() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.level
}

func (c *Client) TickRate() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tickRate
}

// LatestSnapshot returns the most recent WorldSnapshot, or nil. Non-blocking.
func (c *Client) LatestSnapshot() *esync.WorldSnapshot {
	select {
	case snap := <-c.snapshotCh:
		return &snap
	default:
		return nil
	}
}

func (c *Client) SendMessage(msg any) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	payload, err := router.Serialize(msg)
	if err != nil {
		return fmt.Errorf("serialize: %w", err)
	}

	return conn.Write(context.Background(), websocket.MessageBinary, payload)
}

func (c *Client) setError(err error) {
	c.mu.Lock()
	c.state = StateError
	c.lastError = err
	c.mu.Unlock()
}

// DrainChargeEvents returns all pending charge events, non-blocking.
func (c *Client) DrainChargeEvents() []messages.BoomerangChargeEvent {
	return drainChan(c.chargeCh)
}

// DrainThrowEvents returns all pending throw events, non-blocking.
func (c *Client) DrainThrowEvents() []messages.BoomerangThrowEvent {
	return drainChan(c.throwCh)
}

// DrainCatchEvents returns all pending catch events, non-blocking.
func (c *Client) DrainCatchEvents() []messages.BoomerangCatchEvent {
	return drainChan(c.catchCh)
}

// DrainHitEvents returns all pending hit events, non-blocking.
func (c *Client) DrainHitEvents() []messages.BoomerangHitEvent {
	return drainChan(c.hitCh)
}

func drainChan[T any](ch chan T) []T {
	var out []T
	for {
		select {
		case v := <-ch:
			out = append(out, v)
		default:
			return out
		}
	}
}
