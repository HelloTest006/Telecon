package signal

// Envelope is a signaling message (SDP/ICE only — never app payload).
type Envelope struct {
	Type     string `json:"type"` // offer | answer | ice | hello | bye | error
	From     string `json:"from"`
	To       string `json:"to,omitempty"`
	Room     string `json:"room,omitempty"` // usually sorted device pair
	SDP      string `json:"sdp,omitempty"`
	ICE      string `json:"ice,omitempty"` // JSON candidate
	Error    string `json:"error,omitempty"`
}

// RoomID builds a stable room for two device ids.
func RoomID(a, b string) string {
	if a < b {
		return a + "|" + b
	}
	return b + "|" + a
}
