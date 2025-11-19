package surreal

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	conn      *websocket.Conn
	host      string
	user      string
	pass      string
	namespace string
	database  string
	mu        sync.Mutex
	pending   map[string]chan Response
	closed    chan struct{}
}

type Request struct {
	ID     string        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewClient(host, user, pass, namespace, database string) (*Client, error) {
	c := &Client{
		host:      host,
		user:      user,
		pass:      pass,
		namespace: namespace,
		database:  database,
		pending:   make(map[string]chan Response),
		closed:    make(chan struct{}),
	}

	// Start readLoop BEFORE connect() so RPC calls can receive responses
	go c.readLoop()
	go c.maintainConnection()

	if err := c.connect(); err != nil {
		close(c.closed)
		return nil, err
	}

	return c, nil
}

func (c *Client) connect() error {
	c.mu.Lock()

	if c.conn != nil {
		c.conn.Close()
	}

	// SurrealDB 2.0 requires specifying the protocol format (JSON or CBOR)
	// We'll use JSON for simplicity
	dialer := websocket.Dialer{
		Subprotocols: []string{"json"},
	}

	conn, _, err := dialer.Dial(c.host, nil)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to dial: %w", err)
	}
	c.conn = conn
	c.mu.Unlock() // Unlock before making RPC calls

	// Authenticate
	if err := c.rpcCall("signin", []interface{}{map[string]interface{}{
		"user": c.user,
		"pass": c.pass,
	}}, nil); err != nil {
		conn.Close()
		return fmt.Errorf("signin failed: %w", err)
	}

	// Use NS/DB
	if err := c.rpcCall("use", []interface{}{c.namespace, c.database}, nil); err != nil {
		conn.Close()
		return fmt.Errorf("use failed: %w", err)
	}

	return nil
}

func (c *Client) Close() {
	close(c.closed)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) rpcCall(method string, params []interface{}, result interface{}) error {
	id := uuid.New().String()
	req := Request{
		ID:     id,
		Method: method,
		Params: params,
	}

	respChan := make(chan Response, 1)
	c.mu.Lock()
	if c.conn == nil {
		c.mu.Unlock()
		return fmt.Errorf("connection closed")
	}
	c.pending[id] = respChan
	err := c.conn.WriteJSON(req)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("write failed: %w", err)
	}

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result failed: %w", err)
			}
		}
		return nil
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return fmt.Errorf("timeout")
	}
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.closed:
			return
		default:
			// Continue
		}

		var resp Response
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if err := conn.ReadJSON(&resp); err != nil {
			log.Printf("Read error: %v", err)
			c.reconnect()
			continue
		}

		c.mu.Lock()
		if ch, ok := c.pending[resp.ID]; ok {
			ch <- resp
			delete(c.pending, resp.ID)

		}
		c.mu.Unlock()
	}
}

func (c *Client) reconnect() {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()

	for {
		select {
		case <-c.closed:
			return
		default:
			log.Println("Reconnecting to SurrealDB...")
			if err := c.connect(); err == nil {
				log.Println("Reconnected!")
				return
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *Client) maintainConnection() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			// Ping
			// SurrealDB doesn't have a standard "ping" method in RPC?
			// We can run a simple query "INFO FOR DB;" or just "RETURN true;"
			// Or use "ping" method if supported.
			// Let's use "ping" method as per some docs, or fallback to query.
			// Actually, "ping" is supported.
			if err := c.rpcCall("ping", nil, nil); err != nil {
				log.Printf("Ping failed: %v", err)
				// Reconnect is handled by readLoop error or we can trigger it here
				// But readLoop handles connection errors.
			}
		}
	}
}

func (c *Client) Query(sql string, vars interface{}) (interface{}, error) {
	var result interface{}
	err := c.rpcCall("query", []interface{}{sql, vars}, &result)
	return result, err
}

func (c *Client) Create(thing string, data interface{}) (interface{}, error) {
	var result interface{}
	err := c.rpcCall("create", []interface{}{thing, data}, &result)
	return result, err
}

func (c *Client) Select(thing string) (interface{}, error) {
	var result interface{}
	err := c.rpcCall("select", []interface{}{thing}, &result)
	return result, err
}
