package protocol

type NostrAddress struct {
	Nprofile string
	pubkey   string
}

func (n NostrAddress) String() string {
	return n.Nprofile
}
func (n NostrAddress) Network() string {
	return "nostr"
}
