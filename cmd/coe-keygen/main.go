package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	coecrypto "github.com/telecon/coe/internal/crypto"
	"github.com/telecon/coe/internal/device"
)

func main() {
	deviceID := flag.String("device-id", "", "device id (required)")
	out := flag.String("out", "", "identity JSON path (default data/devices/<id>/identity.json)")
	profile := flag.String("profile", "strong", "strong|simple")
	org := flag.String("org", "default", "org id for simple profile")
	flag.Parse()
	if *deviceID == "" {
		log.Fatal("-device-id required")
	}
	path := *out
	if path == "" {
		path = filepath.Join("data", "devices", *deviceID, "identity.json")
	}
	sk, pk, err := coecrypto.GenerateX25519()
	if err != nil {
		log.Fatal(err)
	}
	signPK, signSK, err := device.GenerateSignKey()
	if err != nil {
		log.Fatal(err)
	}
	id := device.Identity{
		DeviceID:          *deviceID,
		EnrollmentCounter: 1,
		PrivateKey:        sk,
		PublicKey:         pk,
		SignPrivate:       signSK,
		SignPublic:        signPK,
		OrgID:             *org,
		Profile:           *profile,
	}
	if err := device.SaveIdentity(path, id); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s (DPAPI=%v)\n", path, device.ProtectAvailable())
	fmt.Printf("device_id=%s\nidentity_pk=%s\nsign_pk=%s\n",
		*deviceID, hex.EncodeToString(pk), hex.EncodeToString(signPK))
}
