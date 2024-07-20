package netstr

// NostrAddress represents a type that holds the profile and public key of a Nostr address.
type NostrAddress struct {
	Nprofile string
	pubkey   string
}

func (n NostrAddress) String() string {
	return n.Nprofile
}

// Network returns the network type of the NostrAddress, which is "nostr".
func (n NostrAddress) Network() string {
	return "nostr"
}
