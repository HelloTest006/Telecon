package p2p

const (
	TypeHello     uint8 = 1
	TypeHelloAck  uint8 = 2
	TypeSessionOK uint8 = 3
	TypeData      uint8 = 4
	TypeClose     uint8 = 5
	TypeEpochWarn uint8 = 6
	TypeKeyOffer  uint8 = 7 // sealed GeneralKey under wrap key
)

type Hello struct {
	Type         uint8  `json:"type"`
	Ver          uint8  `json:"ver"`
	DeviceID     string `json:"device_id"`
	EpochID      uint64 `json:"epoch_id"`
	IdentityPK   []byte `json:"identity_pk"`
	EphPK        []byte `json:"eph_pk,omitempty"`
	SessionNonce []byte `json:"session_nonce"`
	KeySerial    uint64 `json:"key_serial"`
	Profile      string `json:"profile"`
}

type HelloAck struct {
	Type         uint8  `json:"type"`
	Ver          uint8  `json:"ver"`
	DeviceID     string `json:"device_id"`
	EpochID      uint64 `json:"epoch_id"`
	IdentityPK   []byte `json:"identity_pk"`
	EphPK        []byte `json:"eph_pk,omitempty"`
	SessionNonce []byte `json:"session_nonce"`
	KeySerial    uint64 `json:"key_serial"`
	Profile      string `json:"profile"`
	Accept       bool   `json:"accept"`
}

// KeyOffer carries sealed daily general key (type 7).
type KeyOffer struct {
	Type       uint8  `json:"type"`
	EpochID    uint64 `json:"epoch_id"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type DataMsg struct {
	Type       uint8  `json:"type"`
	EpochID    uint64 `json:"epoch_id"`
	Seq        uint64 `json:"seq"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

type CloseMsg struct {
	Type   uint8  `json:"type"`
	Reason string `json:"reason"`
}
