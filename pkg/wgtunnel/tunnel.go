// Package wgtunnel implements a DERP-based secure relay tunnel that uses
// WireGuard node keys for end-to-end encryption and authentication.
//
// Traffic flow:
//
//	CLI stdin/stdout ←→ DERPRelay ←→ DERP server ←→ DERPRelay ←→ Agent ←→ localTarget
//
// DERP encrypts all messages with box encryption keyed by the WireGuard
// node keys, so the server cannot read the payload.  Any server replica
// can forward DERP frames (stateless routing by public key).
package wgtunnel

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"tailscale.com/derp"
	"tailscale.com/derp/derphttp"
	"tailscale.com/types/key"
	tslogger "tailscale.com/types/logger"
)

// ─── message types ────────────────────────────────────────────────────────────

const (
	msgData  byte = 0x01 // payload: raw TCP data
	msgClose byte = 0x03 // no payload; signals end of stream
)

// ─── DERPRelay ────────────────────────────────────────────────────────────────

// DERPRelay wraps a derphttp.Client for a single peer-to-peer relay session.
// All data sent/received is encrypted by DERP using the node private keys.
type DERPRelay struct {
	client  *derphttp.Client
	peerKey key.NodePublic
	cancel  context.CancelFunc
	ctx     context.Context
}

// NewDERPRelay connects to derpURL (e.g. "https://frp.byai.uk/derp") using
// privKey, and targets messages to peerKey.
func NewDERPRelay(privKey key.NodePrivate, derpURL string, peerKey key.NodePublic) (*DERPRelay, error) {
	client, err := derphttp.NewClient(privKey, derpURL, tslogger.Discard)
	if err != nil {
		return nil, fmt.Errorf("derphttp.NewClient: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := client.Connect(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("DERP connect: %w", err)
	}
	return &DERPRelay{
		client:  client,
		peerKey: peerKey,
		cancel:  cancel,
		ctx:     ctx,
	}, nil
}

// Close terminates the DERP connection.
func (r *DERPRelay) Close() {
	r.cancel()
	_ = r.client.Close()
}

// send sends a framed message to peerKey.
func (r *DERPRelay) send(msgType byte, data []byte) error {
	frame := make([]byte, 1+len(data))
	frame[0] = msgType
	copy(frame[1:], data)
	return r.client.Send(r.peerKey, frame)
}

// recvFiltered reads the next message from peerKey (ignoring other senders).
func (r *DERPRelay) recvFiltered() (msgType byte, data []byte, err error) {
	for {
		msg, err := r.client.Recv()
		if err != nil {
			return 0, nil, err
		}
		pkt, ok := msg.(derp.ReceivedPacket)
		if !ok || pkt.Source != r.peerKey || len(pkt.Data) == 0 {
			continue
		}
		return pkt.Data[0], pkt.Data[1:], nil
	}
}

// ─── AgentListener ────────────────────────────────────────────────────────────

// AgentListener runs on the agent side, accepting exactly one DERP relay
// connection per session and bridging it to the local target service.
type AgentListener struct {
	privKey     key.NodePrivate
	derpURL     string
	localTarget string
}

// NewAgentListener creates an AgentListener.
func NewAgentListener(privKey key.NodePrivate, derpURL, localTarget string) *AgentListener {
	return &AgentListener{privKey: privKey, derpURL: derpURL, localTarget: localTarget}
}

// Accept blocks until a DERP message from peerKey is received (the CLI's
// connect signal), then bridges the session to localTarget.
// Call Accept in a goroutine for each expected CLI connection.
func (a *AgentListener) Accept(peerKey key.NodePublic) error {
	relay, err := NewDERPRelay(a.privKey, a.derpURL, peerKey)
	if err != nil {
		return fmt.Errorf("NewDERPRelay: %w", err)
	}
	defer relay.Close()

	// Wait for the CLI's first data or connect message.
	msgType, data, err := relay.recvFiltered()
	if err != nil {
		return fmt.Errorf("waiting for first message: %w", err)
	}
	if msgType == msgClose {
		return nil // CLI disconnected before sending data
	}

	// Dial the local target service.
	var localConn net.Conn
	deadline := time.Now().Add(60 * time.Second)
	for {
		localConn, err = net.Dial("tcp", a.localTarget)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dial %s: %w", a.localTarget, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	defer localConn.Close()

	log.Printf("DERP tunnel: bridging to %s (peer: %s)", a.localTarget, peerKey.ShortString())

	// Forward the first chunk that arrived before we opened the local conn.
	if len(data) > 0 {
		if _, err := localConn.Write(data); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(relay.ctx)
	defer cancel()

	// localTarget → DERP peer
	go func() {
		defer cancel()
		buf := make([]byte, 64*1024)
		for {
			n, err := localConn.Read(buf)
			if n > 0 {
				if sErr := relay.send(msgData, buf[:n]); sErr != nil {
					return
				}
			}
			if err != nil {
				_ = relay.send(msgClose, nil)
				return
			}
		}
	}()

	// DERP peer → localTarget
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		mt, payload, err := relay.recvFiltered()
		if err != nil || mt == msgClose {
			return err
		}
		if len(payload) > 0 {
			if _, err := localConn.Write(payload); err != nil {
				return err
			}
		}
	}
}

// ─── CLIDialer ────────────────────────────────────────────────────────────────

// CLIDialer connects to an agent via DERP relay and exposes the connection as
// an io.ReadWriteCloser suitable for bridging to stdin/stdout (SSH ProxyCommand).
type CLIDialer struct {
	relay *DERPRelay
}

// Dial connects to the agent identified by agentPubKey through derpURL.
// The returned *CLIDialer bridges reads/writes to/from the agent.
func Dial(privKey key.NodePrivate, derpURL string, agentPubKey key.NodePublic) (*CLIDialer, error) {
	relay, err := NewDERPRelay(privKey, derpURL, agentPubKey)
	if err != nil {
		return nil, err
	}

	// Send an empty data frame to signal "I'm here, open your local conn".
	if err := relay.send(msgData, nil); err != nil {
		relay.Close()
		return nil, fmt.Errorf("send connect signal: %w", err)
	}

	return &CLIDialer{relay: relay}, nil
}

// Close terminates the relay and signals the agent to close.
func (d *CLIDialer) Close() error {
	_ = d.relay.send(msgClose, nil)
	d.relay.Close()
	return nil
}

// BridgeStdio copies between the given reader/writer and the DERP relay until
// either side closes.  Blocks until done.
func (d *CLIDialer) BridgeStdio(r io.Reader, w io.Writer) {
	ctx, cancel := context.WithCancel(d.relay.ctx)
	defer cancel()

	// stdin → DERP agent
	go func() {
		defer cancel()
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				if sErr := d.relay.send(msgData, buf[:n]); sErr != nil {
					return
				}
			}
			if err != nil {
				_ = d.relay.send(msgClose, nil)
				return
			}
		}
	}()

	// DERP agent → stdout
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		mt, payload, err := d.relay.recvFiltered()
		if err != nil || mt == msgClose {
			return
		}
		if len(payload) > 0 {
			if _, werr := w.Write(payload); werr != nil {
				return
			}
		}
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// ParsePublicKey decodes a hex-encoded WireGuard public key (64 chars)
// or a tailscale "nodekey:<hex>" string into a key.NodePublic.
func ParsePublicKey(s string) (key.NodePublic, error) {
	pub, err := nodePublicFromHex(s)
	if err != nil {
		// Try parsing as tailscale text format "nodekey:…"
		var k key.NodePublic
		if uerr := k.UnmarshalText([]byte(s)); uerr != nil {
			return key.NodePublic{}, fmt.Errorf("bad public key %q: hex: %v; text: %v", s, err, uerr)
		}
		return k, nil
	}
	return pub, nil
}

// encodeUint32 encodes n as 4 little-endian bytes.
func encodeUint32(n uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, n)
	return b
}
