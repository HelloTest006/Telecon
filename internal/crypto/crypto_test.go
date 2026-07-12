package coecrypto

import (
	"bytes"
	"testing"
	"time"
)

func TestGeneralKeyDeterministic(t *testing.T) {
	master := bytes.Repeat([]byte{0x42}, 32)
	k1, err := DeriveGeneralKey(master, 100, "dev-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := DeriveGeneralKey(master, 100, "dev-a", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("not deterministic")
	}
	k3, _ := DeriveGeneralKey(master, 101, "dev-a", 1)
	if bytes.Equal(k1, k3) {
		t.Fatal("epoch should change key")
	}
}

func TestAEADRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	seq := uint64(3)
	nonce := NonceFromSeq(seq)
	aad := BuildAAD(1, 50, "a", "b", seq)
	ct, err := Seal(key, nonce, aad, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := Open(key, nonce, aad, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != "hi" {
		t.Fatalf("got %q", pt)
	}
}

func TestAgreeEpoch(t *testing.T) {
	// fixed time: epoch 1000 -> unix 1000*86400
	now := time.Unix(1000*86400+10, 0).UTC()
	ep, ok := AgreeEpoch(1000, 1000, now, 300)
	if !ok || ep != 1000 {
		t.Fatalf("got %d %v", ep, ok)
	}
}

func TestSessionSymmetric(t *testing.T) {
	skA, pkA, _ := GenerateX25519()
	skB, pkB, _ := GenerateX25519()
	gkA := bytes.Repeat([]byte{1}, 32)
	gkB := bytes.Repeat([]byte{2}, 32)
	nA := bytes.Repeat([]byte{3}, 16)
	nB := bytes.Repeat([]byte{4}, 16)
	_, sendA, recvA, err := DeriveSessionStrong(SessionParams{
		EpochID: 5, DeviceIDA: "a", DeviceIDB: "b",
		GeneralKeyA: gkA, GeneralKeyB: gkB,
		IdentitySk: skA, IdentityPkPeer: pkB,
		NonceA: nA, NonceB: nB, Profile: "strong",
		LocalDeviceID: "a", PeerDeviceID: "b",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, sendB, recvB, err := DeriveSessionStrong(SessionParams{
		EpochID: 5, DeviceIDA: "a", DeviceIDB: "b",
		GeneralKeyA: gkA, GeneralKeyB: gkB,
		IdentitySk: skB, IdentityPkPeer: pkA,
		NonceA: nA, NonceB: nB, Profile: "strong",
		LocalDeviceID: "b", PeerDeviceID: "a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sendA, recvB) {
		t.Fatal("A send should equal B recv")
	}
	if !bytes.Equal(sendB, recvA) {
		t.Fatal("B send should equal A recv")
	}
}

func TestWrapKeySymmetric(t *testing.T) {
	skA, pkA, _ := GenerateX25519()
	skB, pkB, _ := GenerateX25519()
	eA, epA, _ := GenerateX25519()
	eB, epB, _ := GenerateX25519()
	nA := bytes.Repeat([]byte{3}, 16)
	nB := bytes.Repeat([]byte{4}, 16)
	wA, err := DeriveWrapKey(skA, pkB, eA, epB, nA, nB, 9, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	wB, err := DeriveWrapKey(skB, pkA, eB, epA, nA, nB, 9, "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wA, wB) {
		t.Fatal("wrap keys differ")
	}
}
