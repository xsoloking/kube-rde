package wgtunnel

import (
	"encoding/hex"
	"fmt"
	"strings"

	"go4.org/mem"
	"tailscale.com/types/key"
)

// nodePublicFromHex parses a 64-char lowercase hex string into a NodePublic.
func nodePublicFromHex(s string) (key.NodePublic, error) {
	raw, err := hex.DecodeString(s)
	if err != nil {
		return key.NodePublic{}, fmt.Errorf("hex decode: %w", err)
	}
	if len(raw) != 32 {
		return key.NodePublic{}, fmt.Errorf("expected 32 bytes, got %d", len(raw))
	}
	return key.NodePublicFromRaw32(mem.B(raw)), nil
}

// PrivKeyToHex converts a NodePrivate to the bare 64-char hex form that
// WireGuard's UAPI (IpcSet) expects for private_key=.
func PrivKeyToHex(k key.NodePrivate) string {
	text, _ := k.MarshalText() // "privkey:<64hex>"
	return strings.TrimPrefix(string(text), "privkey:")
}

// PubKeyToHex converts a NodePublic to the bare 64-char hex form used as
// WireGuard's UAPI public_key= and as the ParseEndpoint identifier.
func PubKeyToHex(k key.NodePublic) string {
	r := k.Raw32()
	return hex.EncodeToString(r[:])
}
