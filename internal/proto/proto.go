// Package proto holds the messages two peers send each other.
package proto

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/thyfriendlyfox/lynx/internal/index"
)

// Protocol identifiers. The number changes when the messages change in a way
// that an older peer cannot read.
const (
	Version      = 1
	SyncProtocol = "/lynx/1/sync"
	BlobProtocol = "/lynx/1/blob"
)

// Message types.
const (
	TypeHello    = "hello"
	TypeAuth     = "auth"
	TypeManifest = "manifest"
	TypeUpdate   = "update"
	TypePing     = "ping"
)

// Message is one line on a sync stream.
type Message struct {
	Type     string        `json:"type"`
	Proto    int           `json:"proto,omitempty"`
	PeerName string        `json:"peer,omitempty"`
	FolderID string        `json:"folder,omitempty"`
	Nonce    string        `json:"nonce,omitempty"`
	Proof    string        `json:"proof,omitempty"`
	Entries  []index.Entry `json:"entries,omitempty"`
	Entry    *index.Entry  `json:"entry,omitempty"`
}

// BlobRequest asks a peer for the bytes of one file.
type BlobRequest struct {
	Path     string `json:"path"`
	Version  uint64 `json:"version"`
	FolderID string `json:"folder"`
	Proof    string `json:"proof"`
	Nonce    string `json:"nonce"`
}

// BlobHeader answers a BlobRequest. The bytes follow the header on the same
// stream.
type BlobHeader struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Size  int64  `json:"size"`
	Hash  string `json:"hash"`
}

// ErrClosed reports a stream that ended.
var ErrClosed = errors.New("stream closed")

// Conn frames JSON messages on a stream. One message is one line, so raw
// bytes can follow a message on the same stream.
type Conn struct {
	r  *bufio.Reader
	w  *bufio.Writer
	mu sync.Mutex
	rw io.ReadWriteCloser
}

// NewConn wraps a stream.
func NewConn(rw io.ReadWriteCloser) *Conn {
	return &Conn{
		r:  bufio.NewReaderSize(rw, 64*1024),
		w:  bufio.NewWriterSize(rw, 64*1024),
		rw: rw,
	}
}

// Write sends one message.
func (c *Conn) Write(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := c.w.Write(append(data, '\n')); err != nil {
		return err
	}
	return c.w.Flush()
}

// Read reads one message.
func (c *Conn) Read(v any) error {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && len(line) == 0 {
			return ErrClosed
		}
		if len(line) == 0 {
			return err
		}
	}
	return json.Unmarshal(line, v)
}

// Bytes returns the reader, for the raw bytes that follow a message.
func (c *Conn) Bytes() *bufio.Reader { return c.r }

// Raw returns the writer, for raw bytes that follow a message.
func (c *Conn) Raw() *bufio.Writer { return c.w }

// Close closes the stream.
func (c *Conn) Close() error { return c.rw.Close() }

// Nonce returns a new random challenge as hex.
func Nonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// Proof answers a challenge. It proves that the peer holds the folder key
// without sending the key.
func Proof(key []byte, nonce, folderID string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("lynx-auth:" + folderID + ":" + nonce))
	return hex.EncodeToString(mac.Sum(nil))
}

// CheckProof reports whether an answer to a challenge is correct.
func CheckProof(key []byte, nonce, folderID, proof string) bool {
	want := Proof(key, nonce, folderID)
	return hmac.Equal([]byte(want), []byte(proof))
}
