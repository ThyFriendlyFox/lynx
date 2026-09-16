# Discovery

Discovery is how one copy of a folder finds the other copies. It is the seam
in Lynx that a contributor extends. Lynx has 2 discovery methods today. Both
run at the same time; the first one that works wins.

## mdns

Multicast DNS asks the local network "who runs Lynx?". It needs no internet
and no configuration. It finds a peer in the same house, office or café
network in about 1 second.

A peer found by mDNS still proves that it holds the folder key before any
data moves. A neighbour who runs Lynx sees nothing.

## dht

The public libp2p distributed hash table finds a peer anywhere on the
internet. Lynx publishes a record under the topic `lynx/1/<folder id>` and
looks for the same topic every 45 seconds. The folder id is a hash of the
folder key. The topic never exposes the key.

The first connection over the internet takes 10 seconds to 2 minutes, because
the computer first joins the table. A network that blocks direct connections
in both directions is the known limit of this version; the roadmap holds the
relay work that fixes it.

## Writing a new discovery method

A discovery method gives `internal/p2p` a stream of peer addresses. It
implements the libp2p `discovery.Discoverer` interface, or it calls
`Node.connect` with an address it learned another way.

A pull request with a new method carries 4 things:

1. The implementation in `internal/p2p`.
2. Its section in this file.
3. Its fields in `docs/CONFIGURATION.md`.
4. A gate in `agent-kit/VERIFICATION.md` that proves it connects 2 peers.
