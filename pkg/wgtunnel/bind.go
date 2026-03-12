// Package wgtunnel provides a WireGuard conn.Bind implementation that
// tunnels WireGuard UDP packets through a Tailscale DERP relay.
//
// This enables WireGuard without any real UDP ports or kernel modules:
// both sides connect to the same DERP server over HTTPS and exchange
// encrypted WireGuard packets identified by their WireGuard public keys.
package wgtunnel

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/tailscale/wireguard-go/conn"
	"tailscale.com/derp"
	"tailscale.com/derp/derphttp"
	"tailscale.com/types/key"
	tslogger "tailscale.com/types/logger"
)

// DERPEndpoint implements conn.Endpoint backed by a peer's DERP public key.
// The "destination" of a WireGuard packet is identified by the peer's
// WireGuard public key, which DERP uses to route the packet.
type DERPEndpoint struct {
	peerKey key.NodePublic
}

func (e *DERPEndpoint) ClearSrc()          {}
func (e *DERPEndpoint) SrcToString() string { return "" }
func (e *DERPEndpoint) DstToString() string { return PubKeyToHex(e.peerKey) }
func (e *DERPEndpoint) DstToBytes() []byte  { r := e.peerKey.Raw32(); b := make([]byte, 32); copy(b, r[:]); return b }
func (e *DERPEndpoint) DstIP() netip.Addr   { return netip.Addr{} }
func (e *DERPEndpoint) SrcIP() netip.Addr   { return netip.Addr{} }

// receivedPkt holds an inbound WireGuard packet received from the DERP relay.
type receivedPkt struct {
	source key.NodePublic
	data   []byte
}

// DERPBind implements conn.Bind by relaying WireGuard packets through a
// DERP server.  It requires no UDP sockets and no root privileges.
type DERPBind struct {
	mu     sync.Mutex
	client *derphttp.Client
	pktCh  chan receivedPkt
	cancel context.CancelFunc
	opened bool
}

// NewDERPBind creates a DERPBind that connects to derpServerURL
// (e.g. "https://frp.byai.uk/derp") using the given WireGuard private key.
func NewDERPBind(privateKey key.NodePrivate, derpServerURL string) (*DERPBind, error) {
	client, err := derphttp.NewClient(privateKey, derpServerURL, tslogger.Discard)
	if err != nil {
		return nil, fmt.Errorf("derphttp.NewClient: %w", err)
	}
	return &DERPBind{
		client: client,
		pktCh:  make(chan receivedPkt, 512),
	}, nil
}

// Open connects to the DERP server and starts a receive loop.
// It implements conn.Bind.Open.
func (b *DERPBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.opened {
		return nil, 0, conn.ErrBindAlreadyOpen
	}
	b.opened = true

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel

	if err := b.client.Connect(ctx); err != nil {
		cancel()
		b.opened = false
		return nil, 0, fmt.Errorf("DERP connect: %w", err)
	}

	go b.recvLoop(ctx)

	recvFn := func(packets [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case pkt, ok := <-b.pktCh:
			if !ok {
				return 0, net.ErrClosed
			}
			n := copy(packets[0], pkt.data)
			sizes[0] = n
			eps[0] = &DERPEndpoint{peerKey: pkt.source}
			return 1, nil
		case <-ctx.Done():
			return 0, net.ErrClosed
		}
	}

	return []conn.ReceiveFunc{recvFn}, 0, nil
}

// recvLoop reads packets from DERP and puts them in pktCh.
func (b *DERPBind) recvLoop(ctx context.Context) {
	for {
		msg, err := b.client.Recv()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// On transient errors, try to reconnect once
			if connErr := b.client.Connect(ctx); connErr != nil {
				return
			}
			continue
		}

		pkt, ok := msg.(derp.ReceivedPacket)
		if !ok {
			// Not a data packet (e.g. PeerGoneMessage); skip
			continue
		}

		data := make([]byte, len(pkt.Data))
		copy(data, pkt.Data)

		select {
		case b.pktCh <- receivedPkt{source: pkt.Source, data: data}:
		case <-ctx.Done():
			return
		default:
			// Drop packet if channel is full (backpressure)
		}
	}
}

// Close shuts down the DERP connection and the receive loop.
func (b *DERPBind) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.opened = false
	return b.client.Close()
}

// SetMark is a no-op (DERP uses HTTPS, not raw sockets).
func (b *DERPBind) SetMark(mark uint32) error { return nil }

// Send transmits WireGuard packets through DERP to the target endpoint.
func (b *DERPBind) Send(bufs [][]byte, ep conn.Endpoint, offset int) error {
	derpEP, ok := ep.(*DERPEndpoint)
	if !ok {
		return conn.ErrWrongEndpointType
	}
	for _, buf := range bufs {
		if err := b.client.Send(derpEP.peerKey, buf[offset:]); err != nil {
			return err
		}
	}
	return nil
}

// ParseEndpoint parses a 64-character lowercase hex WireGuard public key
// string and returns a DERPEndpoint for it.
// This is called by wireguard-go when it processes the `endpoint=` line
// from IpcSet; we encode peer public keys as hex strings there.
func (b *DERPBind) ParseEndpoint(s string) (conn.Endpoint, error) {
	pub, err := nodePublicFromHex(s)
	if err != nil {
		return nil, fmt.Errorf("invalid DERP endpoint %q: %w", s, err)
	}
	return &DERPEndpoint{peerKey: pub}, nil
}

// BatchSize reports one packet per call (DERP is stream-based, not batch).
func (b *DERPBind) BatchSize() int { return 1 }
