package snapws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
)

const (
	OpcodeContinuation = 0
	OpcodeText         = 1
	OpcodeBinary       = 2
	OpcodeClose        = 8
	OpcodePing         = 9
	OpcodePong         = 10

	CloseNormalClosure             = 1000
	CloseGoingAway                 = 1001
	CloseProtocolError             = 1002
	CloseUnsupportedData           = 1003
	CloseInvalidFramePayloadData   = 1007
	ClosePolicyViolation           = 1008
	CloseMessageTooBig             = 1009
	CloseInternalServerErr         = 1011

	MaxControlFramePayload = 125
)

var (
	ErrConnClosed            = &FatalError{err: "connection closed"}
	ErrInvalidOPCODE         = &FatalError{err: "invalid opcode"}
	ErrMessageTypeMismatch   = &FatalError{err: "message type mismatch"}
	ErrPingAlreadySent       = &FatalError{err: "ping already sent"}
	ErrInternalServer        = &FatalError{err: "internal server error"}
	ErrWriterUnintialized    = &FatalError{err: "writer uninitialized"}
	ErrWriterClosed          = &FatalError{err: "writer closed"}
	ErrTooLargePayload       = &FatalError{err: "payload too large"}
	ErrMessageTooLarge       = &FatalError{err: "message too large"}
	ErrTooMuchFragments      = &FatalError{err: "too many fragments"}
	ErrInvalidUTF8           = &FatalError{err: "invalid utf-8"}
	ErrExpectedContinuation  = &FatalError{err: "expected continuation frame"}
	ErrUnnegotiatedRsvBits   = &FatalError{err: "unnegotiated rsv bits"}
)

type FatalError struct {
	err string
}

func (e *FatalError) Error() string {
	return e.err
}

func fatal(err error) error {
	if _, ok := err.(*FatalError); ok {
		return err
	}
	return &FatalError{err: err.Error()}
}

type BackpressureStrategy int

const (
	BackpressureDrop BackpressureStrategy = iota
	BackpressureClose
	BackpressureWait
)

type Options struct {
	Middlwares              []Middlware
	OnConnect               func(conn *Conn)
	OnDisconnect            func(conn *Conn)
	WriteWait               time.Duration
	ReadWait                time.Duration
	PingEvery               time.Duration
	MaxMessageSize          int
	ReaderMaxFragments      int
	ReadBufferSize          int
	WriteBufferSize         int
	DisableWriteBuffersPooling bool
	SubProtocols            []string
	RejectRaw               bool
	BroadcastChannelsSize   int
	BroadcastBackpressure   BackpressureStrategy
	SkipUTF8Validation      bool
	MaxBatchSize            int
}

type Middlware func(w http.ResponseWriter, r *http.Request) error

type Upgrader struct {
	opts *Options
}

func NewUpgrader(opts *Options) *Upgrader {
	if opts == nil {
		opts = &Options{}
	}
	if opts.WriteWait == 0 {
		opts.WriteWait = 5 * time.Second
	}
	if opts.ReadWait == 0 {
		opts.ReadWait = 60 * time.Second
	}
	if opts.PingEvery == 0 {
		opts.PingEvery = 50 * time.Second
	}
	if opts.MaxMessageSize == 0 {
		opts.MaxMessageSize = 1 << 20
	}
	if opts.WriteBufferSize == 0 {
		opts.WriteBufferSize = 4096
	}
	if opts.ReadBufferSize == 0 {
		opts.ReadBufferSize = 4096
	}
	if opts.BroadcastChannelsSize == 0 {
		opts.BroadcastChannelsSize = 8
	}
	return &Upgrader{opts: opts}
}

func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	for _, mw := range u.opts.Middlwares {
		if err := mw(w, r); err != nil {
			return nil, err
		}
	}

	acceptOptions := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	}

	if len(u.opts.SubProtocols) > 0 {
		acceptOptions.Subprotocols = u.opts.SubProtocols
	}

	rawConn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		return nil, err
	}

	conn := &Conn{
		rawConn:  rawConn,
		upgrader: u,
		done:     make(chan struct{}),
	}

	if u.opts.OnConnect != nil {
		u.opts.OnConnect(conn)
	}

	go conn.pingLoop()

	return conn, nil
}

type Conn struct {
	rawConn  *websocket.Conn
	upgrader *Upgrader
	done     chan struct{}
	closeOnce sync.Once
	pingSent bool
}

func (c *Conn) pingLoop() {
	ticker := time.NewTicker(c.upgrader.opts.PingEvery)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-c.done:
			return
		default:
			if err := c.Ping(); err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.rawConn.Close(websocket.StatusNormalClosure, "")
		if c.upgrader.opts.OnDisconnect != nil {
			c.upgrader.opts.OnDisconnect(c)
		}
	})
}

func (c *Conn) SendJSON(ctx context.Context, v any) error {
	select {
	case <-c.done:
		return ErrConnClosed
	default:
	}

	writeCtx, cancel := context.WithTimeout(ctx, c.upgrader.opts.WriteWait)
	defer cancel()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return c.rawConn.Write(writeCtx, websocket.MessageText, data)
}

func (c *Conn) SendBytes(ctx context.Context, p []byte) error {
	select {
	case <-c.done:
		return ErrConnClosed
	default:
	}

	writeCtx, cancel := context.WithTimeout(ctx, c.upgrader.opts.WriteWait)
	defer cancel()

	return c.rawConn.Write(writeCtx, websocket.MessageBinary, p)
}

func (c *Conn) ReadJSON(v any) error {
	select {
	case <-c.done:
		return ErrConnClosed
	default:
	}

	readCtx, cancel := context.WithTimeout(context.Background(), c.upgrader.opts.ReadWait)
	defer cancel()

	_, data, err := c.rawConn.Read(readCtx)
	if err != nil {
		return err
	}

	if !c.upgrader.opts.SkipUTF8Validation && !utf8.Valid(data) {
		c.Close()
		return ErrInvalidUTF8
	}

	return json.Unmarshal(data, v)
}

func (c *Conn) ReadBinary() ([]byte, error) {
	select {
	case <-c.done:
		return nil, ErrConnClosed
	default:
	}

	readCtx, cancel := context.WithTimeout(context.Background(), c.upgrader.opts.ReadWait)
	defer cancel()

	msgType, data, err := c.rawConn.Read(readCtx)
	if err != nil {
		return nil, err
	}

	if msgType != websocket.MessageBinary {
		return nil, ErrMessageTypeMismatch
	}

	return data, nil
}

func (c *Conn) Ping() error {
	if c.pingSent {
		return ErrPingAlreadySent
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), c.upgrader.opts.WriteWait)
	defer cancel()

	c.pingSent = true
	return c.rawConn.Ping(pingCtx)
}

func (c *Conn) Pong(data []byte) error {
	pongCtx, cancel := context.WithTimeout(context.Background(), c.upgrader.opts.WriteWait)
	defer cancel()

	return c.rawConn.Ping(pongCtx)
}

func (c *Conn) MetaData() *sync.Map {
	return &sync.Map{}
}

func (c *Conn) SubProtocol() string {
	return ""
}

type MiddlewareErr struct {
	Code    int
	Message string
}

func (e *MiddlewareErr) Error() string {
	return e.Message
}

func NewMiddlewareErr(code int, message string) *MiddlewareErr {
	return &MiddlewareErr{Code: code, Message: message}
}

func AsMiddlewareErr(err error) (*MiddlewareErr, bool) {
	e, ok := err.(*MiddlewareErr)
	return e, ok
}

type RateLimiter struct {
	mu sync.Mutex
}

func (rl *RateLimiter) addClient(conn *Conn) {}
func (rl *RateLimiter) removeClient(conn *Conn) {}

type batchFlusher struct{}

func (bf *batchFlusher) remove(conn *Conn) {}
func (bf *batchFlusher) newBatch(conn *Conn) *messageBatch { return nil }

type messageBatch struct{}

type pooledBuf struct {
	buf []byte
}

type mu struct {
	mu sync.Mutex
}

func newMu(conn *Conn) *mu {
	return &mu{}
}

func (m *mu) lockCtx(ctx context.Context) error {
	m.mu.Lock()
	return nil
}

func (m *mu) unLock() {
	m.mu.Unlock()
}

func (m *mu) tryUnlock() {
	m.mu.Unlock()
}

func (m *mu) forceLock() {
	m.mu.Lock()
}

func (m *mu) lockTimer(t *time.Timer) error {
	m.mu.Lock()
	return nil
}

type ConnReader struct {
	conn *Conn
}

type ConnWriter struct {
	conn *Conn
}

type ControlWriter struct {
	conn *Conn
}

func isData(opcode uint8) bool {
	return opcode == OpcodeText || opcode == OpcodeBinary
}

func isControl(opcode uint8) bool {
	return opcode == OpcodeClose || opcode == OpcodePing || opcode == OpcodePong
}

func isVaidOpcode(opcode uint8) bool {
	return opcode <= OpcodeBinary || (opcode >= OpcodeClose && opcode <= OpcodePong)
}

func isValidCloseCode(code uint16) bool {
	return (code >= 1000 && code <= 1003) || (code >= 1007 && code <= 1011)
}

func comparePayload(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type Manager[KeyType comparable] struct {
	conns        map[KeyType]*ManagedConn[KeyType]
	mu           sync.RWMutex
	Upgrader     *Upgrader
	OnRegister   func(conn *ManagedConn[KeyType])
	OnUnregister func(conn *ManagedConn[KeyType])
}

func NewManager[KeyType comparable](u *Upgrader) *Manager[KeyType] {
	if u == nil {
		u = NewUpgrader(nil)
	}
	return &Manager[KeyType]{
		conns:    make(map[KeyType]*ManagedConn[KeyType]),
		Upgrader: u,
	}
}

type ManagedConn[KeyType comparable] struct {
	*Conn
	Key     KeyType
	Manager *Manager[KeyType]
}

func (m *Manager[KeyType]) Connect(key KeyType, w http.ResponseWriter, r *http.Request) (*ManagedConn[KeyType], error) {
	c, err := m.Upgrader.Upgrade(w, r)
	if err != nil {
		return nil, err
	}
	conn := &ManagedConn[KeyType]{Conn: c, Key: key, Manager: m}
	m.Register(key, conn)
	return conn, nil
}

func (m *Manager[KeyType]) Register(key KeyType, conn *ManagedConn[KeyType]) {
	m.mu.Lock()
	if existing, ok := m.conns[key]; ok {
		existing.Close()
	}
	m.conns[key] = conn
	m.mu.Unlock()

	if m.OnRegister != nil {
		m.OnRegister(conn)
	}
}

func (m *Manager[KeyType]) unregister(id KeyType) error {
	m.mu.Lock()
	conn, ok := m.conns[id]
	if !ok {
		m.mu.Unlock()
		return ErrConnClosed
	}
	delete(m.conns, id)
	m.mu.Unlock()

	if m.OnUnregister != nil {
		m.OnUnregister(conn)
	}
	return nil
}

func (m *Manager[KeyType]) Get(key KeyType) *ManagedConn[KeyType] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conns[key]
}

func (m *Manager[KeyType]) GetAllConns(exclude ...KeyType) []*ManagedConn[KeyType] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]*ManagedConn[KeyType], 0, len(m.conns))
	for k, v := range m.conns {
		found := false
		for _, ex := range exclude {
			if k == ex {
				found = true
				break
			}
		}
		if !found {
			conns = append(conns, v)
		}
	}
	return conns
}

func (m *Manager[KeyType]) GetAllConnsAsConn(exclude ...KeyType) []*Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]*Conn, 0, len(m.conns))
	for k, v := range m.conns {
		found := false
		for _, ex := range exclude {
			if k == ex {
				found = true
				break
			}
		}
		if !found {
			conns = append(conns, v.Conn)
		}
	}
	return conns
}

func (m *Manager[KeyType]) Broadcast(ctx context.Context, opcode uint8, data []byte, exclude ...KeyType) error {
	conns := m.GetAllConns(exclude...)
	for _, conn := range conns {
		if opcode == OpcodeText {
			conn.SendJSON(ctx, string(data))
		} else {
			conn.SendBytes(ctx, data)
		}
	}
	return nil
}

func (m *Manager[KeyType]) BroadcastJSON(ctx context.Context, v any, exclude ...KeyType) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return m.Broadcast(ctx, OpcodeText, data, exclude...)
}
