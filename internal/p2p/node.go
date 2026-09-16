// Package p2p connects the folder to the other copies of it.
package p2p

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/thyfriendlyfox/lynx/internal/engine"
	"github.com/thyfriendlyfox/lynx/internal/folder"
	"github.com/thyfriendlyfox/lynx/internal/index"
	"github.com/thyfriendlyfox/lynx/internal/proto"
)

// FindEvery is how often the node looks for peers that hold the same folder.
const FindEvery = 45 * time.Second

// HandshakeTimeout bounds the greeting between two peers.
const HandshakeTimeout = 20 * time.Second

// Node is this machine on the network, for one folder.
type Node struct {
	fol  *folder.Folder
	eng  *engine.Engine
	log  *slog.Logger
	host host.Host
	kdht *dht.IpfsDHT
	disc *drouting.RoutingDiscovery

	mu       sync.Mutex
	sessions map[peer.ID]*session
}

// PeerInfo is what status reports about one connected peer.
type PeerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Since string `json:"since"`
}

// New starts the network side of a folder. It does not connect yet.
func New(fol *folder.Folder, eng *engine.Engine, log *slog.Logger) (*Node, error) {
	priv, err := loadOrCreateKey(fol.KeyPath())
	if err != nil {
		return nil, err
	}
	port := fol.Config.Port
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port),
			fmt.Sprintf("/ip6/::/tcp/%d", port),
			fmt.Sprintf("/ip6/::/udp/%d/quic-v1", port),
		),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
		libp2p.UserAgent("lynx"),
	)
	if err != nil {
		return nil, fmt.Errorf("start the network: %w", err)
	}
	n := &Node{fol: fol, eng: eng, log: log, host: h, sessions: map[peer.ID]*session{}}
	eng.SetFetcher(n)
	eng.SetSender(n.Broadcast)
	h.SetStreamHandler(proto.SyncProtocol, n.handleSync)
	h.SetStreamHandler(proto.BlobProtocol, n.handleBlob)
	return n, nil
}

// Topic is the name the folder has on the network. It comes from the folder
// key by one way, so the topic never exposes the key.
func (n *Node) Topic() string { return "lynx/1/" + n.fol.ID() }

// ID returns the network address of this machine.
func (n *Node) ID() string { return n.host.ID().String() }

// Addrs returns the addresses this machine listens on.
func (n *Node) Addrs() []string {
	out := []string{}
	for _, a := range n.host.Addrs() {
		out = append(out, a.String())
	}
	return out
}

// Start joins the network and keeps looking for peers until ctx ends.
func (n *Node) Start(ctx context.Context) error {
	kdht, err := dht.New(n.host, dht.Mode(dht.ModeAuto), dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...))
	if err != nil {
		return fmt.Errorf("start peer lookup: %w", err)
	}
	n.kdht = kdht
	n.disc = drouting.NewRoutingDiscovery(kdht)

	n.dialBootstrap(ctx)
	if err := kdht.Bootstrap(ctx); err != nil {
		n.log.Warn("bootstrap the peer lookup", "error", err)
	}
	if err := n.startMDNS(); err != nil {
		n.log.Warn("start local network discovery", "error", err)
	}

	go n.findLoop(ctx)
	return nil
}

// Close stops the network.
func (n *Node) Close() error {
	if n.kdht != nil {
		_ = n.kdht.Close()
	}
	return n.host.Close()
}

func (n *Node) dialBootstrap(ctx context.Context) {
	var wg sync.WaitGroup
	for _, info := range dht.GetDefaultBootstrapPeerAddrInfos() {
		wg.Add(1)
		go func(info peer.AddrInfo) {
			defer wg.Done()
			dialCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			if err := n.host.Connect(dialCtx, info); err != nil {
				n.log.Debug("bootstrap peer refused", "peer", info.ID.String(), "error", err)
			}
		}(info)
	}
	wg.Wait()
}

// MDNSService is the name Lynx uses to find peers on the same local network.
const MDNSService = "_lynx._udp"

// startMDNS finds peers on the same local network, with no internet.
func (n *Node) startMDNS() error {
	svc := mdns.NewMdnsService(n.host, MDNSService, &mdnsNotifee{node: n})
	return svc.Start()
}

type mdnsNotifee struct{ node *Node }

// HandlePeerFound connects to a peer seen on the local network. A peer that
// holds another folder key fails the greeting and is dropped.
func (m *mdnsNotifee) HandlePeerFound(ai peer.AddrInfo) {
	if ai.ID == m.node.host.ID() || m.node.hasSession(ai.ID) {
		return
	}
	go m.node.connect(context.Background(), ai)
}

func (n *Node) findLoop(ctx context.Context) {
	topic := n.Topic()
	dutil.Advertise(ctx, n.disc, topic)
	t := time.NewTicker(FindEvery)
	defer t.Stop()
	for {
		n.findOnce(ctx, topic)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (n *Node) findOnce(ctx context.Context, topic string) {
	findCtx, cancel := context.WithTimeout(ctx, FindEvery)
	defer cancel()
	found, err := n.disc.FindPeers(findCtx, topic)
	if err != nil {
		n.log.Debug("look for peers", "error", err)
		return
	}
	for ai := range found {
		if ai.ID == n.host.ID() || len(ai.Addrs) == 0 {
			continue
		}
		if n.hasSession(ai.ID) {
			continue
		}
		go n.connect(ctx, ai)
	}
}

// connect dials a peer and starts a sync session with it.
func (n *Node) connect(ctx context.Context, ai peer.AddrInfo) {
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := n.host.Connect(dialCtx, ai); err != nil {
		n.log.Debug("dial peer", "peer", short(ai.ID), "error", err)
		return
	}
	s, err := n.host.NewStream(dialCtx, ai.ID, proto.SyncProtocol)
	if err != nil {
		n.log.Debug("open sync stream", "peer", short(ai.ID), "error", err)
		return
	}
	conn := proto.NewConn(s)
	name, err := n.greet(conn, true)
	if err != nil {
		n.log.Debug("greet peer", "peer", short(ai.ID), "error", err)
		_ = conn.Close()
		return
	}
	n.runSession(ctx, ai.ID, name, conn)
}

func (n *Node) handleSync(s network.Stream) {
	ctx := context.Background()
	conn := proto.NewConn(s)
	name, err := n.greet(conn, false)
	if err != nil {
		n.log.Debug("greet peer", "peer", short(s.Conn().RemotePeer()), "error", err)
		_ = conn.Close()
		return
	}
	n.runSession(ctx, s.Conn().RemotePeer(), name, conn)
}

// greet proves to the other side that both hold the folder key.
func (n *Node) greet(conn *proto.Conn, initiator bool) (string, error) {
	key := n.fol.Key()
	id := n.fol.ID()
	mine := proto.Nonce()

	if initiator {
		if err := conn.Write(proto.Message{
			Type: proto.TypeHello, Proto: proto.Version,
			PeerName: n.fol.Config.PeerName, FolderID: id, Nonce: mine,
		}); err != nil {
			return "", err
		}
		var reply proto.Message
		if err := conn.Read(&reply); err != nil {
			return "", err
		}
		if reply.FolderID != id || !proto.CheckProof(key, mine, id, reply.Proof) {
			return "", errors.New("the peer does not hold this folder key")
		}
		if err := conn.Write(proto.Message{
			Type: proto.TypeAuth, Proof: proto.Proof(key, reply.Nonce, id),
		}); err != nil {
			return "", err
		}
		return reply.PeerName, nil
	}

	var hello proto.Message
	if err := conn.Read(&hello); err != nil {
		return "", err
	}
	if hello.Type != proto.TypeHello || hello.FolderID != id {
		return "", errors.New("the peer asked for another folder")
	}
	if err := conn.Write(proto.Message{
		Type: proto.TypeHello, Proto: proto.Version,
		PeerName: n.fol.Config.PeerName, FolderID: id, Nonce: mine,
		Proof: proto.Proof(key, hello.Nonce, id),
	}); err != nil {
		return "", err
	}
	var auth proto.Message
	if err := conn.Read(&auth); err != nil {
		return "", err
	}
	if auth.Type != proto.TypeAuth || !proto.CheckProof(key, mine, id, auth.Proof) {
		return "", errors.New("the peer does not hold this folder key")
	}
	return hello.PeerName, nil
}

type session struct {
	id    peer.ID
	name  string
	conn  *proto.Conn
	since time.Time
}

func (n *Node) runSession(ctx context.Context, id peer.ID, name string, conn *proto.Conn) {
	if name == "" {
		name = short(id)
	}
	sess := &session{id: id, name: name, conn: conn, since: time.Now()}

	// Both peers dial each other, so two streams can arrive at once. The
	// first one wins and the second one closes, or the peers flap.
	n.mu.Lock()
	if _, ok := n.sessions[id]; ok {
		n.mu.Unlock()
		_ = conn.Close()
		return
	}
	n.sessions[id] = sess
	n.mu.Unlock()

	n.log.Info("peer connected", "peer", name, "id", short(id))
	defer func() {
		n.mu.Lock()
		if n.sessions[id] == sess {
			delete(n.sessions, id)
		}
		n.mu.Unlock()
		_ = conn.Close()
		n.log.Info("peer disconnected", "peer", name)
	}()

	if err := conn.Write(proto.Message{Type: proto.TypeManifest, Entries: n.eng.Snapshot()}); err != nil {
		n.log.Warn("send manifest", "peer", name, "error", err)
		return
	}

	who := engine.Peer{ID: id.String(), Name: name}
	for {
		var msg proto.Message
		if err := conn.Read(&msg); err != nil {
			return
		}
		switch msg.Type {
		case proto.TypeManifest, proto.TypeUpdate:
			entries := msg.Entries
			if msg.Entry != nil {
				entries = append(entries, *msg.Entry)
			}
			go n.apply(ctx, who, entries, msg.Type == proto.TypeManifest)
		case proto.TypePing:
		}
	}
}

func (n *Node) apply(ctx context.Context, who engine.Peer, entries []index.Entry, full bool) {
	var err error
	if full {
		err = n.eng.ApplyManifest(ctx, who, entries)
	} else {
		for _, e := range entries {
			if applyErr := n.eng.ApplyRemote(ctx, who, e); applyErr != nil && err == nil {
				err = applyErr
			}
		}
	}
	if err != nil {
		n.log.Warn("apply changes from peer", "peer", who.Name, "error", err)
	}
}

// Broadcast tells every connected peer about local changes.
func (n *Node) Broadcast(entries []index.Entry) {
	if len(entries) == 0 {
		return
	}
	msg := proto.Message{Type: proto.TypeUpdate, Entries: entries}
	for _, s := range n.list() {
		if err := s.conn.Write(msg); err != nil {
			n.log.Debug("send update", "peer", s.name, "error", err)
		}
	}
}

// Fetch gets the bytes of one file from one peer.
func (n *Node) Fetch(ctx context.Context, who engine.Peer, e index.Entry, w io.Writer) error {
	id, err := peer.Decode(who.ID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	s, err := n.host.NewStream(ctx, id, proto.BlobProtocol)
	if err != nil {
		return err
	}
	defer s.Close()

	conn := proto.NewConn(s)
	if err := conn.Write(proto.BlobRequest{Path: e.Path, Version: e.Version, FolderID: n.fol.ID()}); err != nil {
		return err
	}
	var head proto.BlobHeader
	if err := conn.Read(&head); err != nil {
		return err
	}
	if !head.OK {
		return errors.New(head.Error)
	}
	if head.Size > engine.MaxFileSize {
		return errors.New("the file is too large to sync")
	}
	if _, err := io.CopyN(w, conn.Bytes(), head.Size); err != nil {
		return err
	}
	return nil
}

func (n *Node) handleBlob(s network.Stream) {
	defer s.Close()
	remote := s.Conn().RemotePeer()
	if !n.hasSession(remote) {
		n.log.Warn("refused a file request from an unknown peer", "peer", short(remote))
		return
	}
	conn := proto.NewConn(s)
	var req proto.BlobRequest
	if err := conn.Read(&req); err != nil {
		return
	}
	if req.FolderID != n.fol.ID() {
		_ = conn.Write(proto.BlobHeader{Error: "wrong folder"})
		return
	}
	f, entry, err := n.eng.OpenFile(req.Path, req.Version)
	if err != nil {
		_ = conn.Write(proto.BlobHeader{Error: err.Error()})
		return
	}
	defer f.Close()
	if err := conn.Write(proto.BlobHeader{OK: true, Size: entry.Size, Hash: entry.Hash}); err != nil {
		return
	}
	if _, err := io.CopyN(conn.Raw(), f, entry.Size); err != nil {
		n.log.Debug("send file", "path", req.Path, "error", err)
		return
	}
	if err := conn.Raw().Flush(); err != nil {
		n.log.Debug("send file", "path", req.Path, "error", err)
	}
}

// Peers returns the connected peers.
func (n *Node) Peers() []PeerInfo {
	out := []PeerInfo{}
	for _, s := range n.list() {
		out = append(out, PeerInfo{ID: s.id.String(), Name: s.name, Since: s.since.Format(time.RFC3339)})
	}
	return out
}

// PeerCount returns how many peers are connected.
func (n *Node) PeerCount() int { return len(n.list()) }

func (n *Node) list() []*session {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]*session, 0, len(n.sessions))
	for _, s := range n.sessions {
		out = append(out, s)
	}
	return out
}

func (n *Node) hasSession(id peer.ID) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.sessions[id]
	return ok
}

func short(id peer.ID) string {
	s := id.String()
	if len(s) <= 8 {
		return s
	}
	return s[len(s)-8:]
}

// loadOrCreateKey returns the identity of this machine for this folder.
func loadOrCreateKey(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return crypto.UnmarshalPrivateKey(data)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	raw, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	return priv, nil
}
