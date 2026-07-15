package kaapi

// Store is the device/voucher registry backend (JSON file or SQLite).
type Store interface {
	Enroll(req EnrollRequest) (*DeviceRecord, error)
	Get(deviceID string) *DeviceRecord
	ListDevices() []DeviceRecord
	Revoke(deviceID string) error
	MarkIssued(deviceID string, epochID uint64) (serial uint64, err error)
	RedeemVoucher(code string) (*Voucher, string)
	AddVoucher(v *Voucher) error
	ListVouchers() []VoucherInfo
	RevokeVoucher(id string) error
}
