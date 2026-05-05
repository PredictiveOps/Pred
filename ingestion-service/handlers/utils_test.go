package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func TestParseDeviceTopic_ValidAndInvalid(t *testing.T) {
	tests := []struct {
		topic    string
		wantID   uint
		wantKind string
		wantErr  bool
	}{
		{"devices/123/data", 123, "data", false},
		{"devices/1/registration", 1, "registration", false},
		{"devices/abc/data", 0, "", true},
		{"wrong/1/data", 0, "", true},
		{"devices/1/unknown", 0, "", true},
	}

	for _, tt := range tests {
		id, kind, err := parseDeviceTopic(tt.topic)
		if (err != nil) != tt.wantErr {
			t.Fatalf("topic %q: unexpected error state: %v", tt.topic, err)
		}
		if tt.wantErr {
			continue
		}
		if id != tt.wantID || kind != tt.wantKind {
			t.Fatalf("topic %q: got (id=%d kind=%s), want (id=%d kind=%s)", tt.topic, id, kind, tt.wantID, tt.wantKind)
		}
	}
}

func TestBuildRegistrationResponseTopic_WithAndWithoutTemplate(t *testing.T) {
	// Without template
	got := buildRegistrationResponseTopic("devices/42/registration", "")
	if got != "devices/42/registration/response" {
		t.Fatalf("unexpected response topic without template: %s", got)
	}

	// With template
	tmpl := "devices/%d/registration/response"
	got2 := buildRegistrationResponseTopic("devices/42/registration", tmpl)
	if got2 != "devices/42/registration/response" {
		t.Fatalf("unexpected response topic with template: %s", got2)
	}
}

func pemFromPublicKey(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	return string(pem.EncodeToMemory(block)), nil
}

func TestParsePublicKeyAndVerifySignature(t *testing.T) {
	// generate a key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pemStr, err := pemFromPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("pemFromPublicKey: %v", err)
	}

	pub, err := ParsePublicKey(pemStr)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}

	payload := []byte("hello world")
	hash := sha256.Sum256(payload)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if !verifySignature(pub, payload, sig) {
		t.Fatal("verifySignature returned false for valid signature")
	}
}

func TestVerifyDeviceData_Succeeds(t *testing.T) {
	// generate key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pemStr, err := pemFromPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("pemFromPublicKey: %v", err)
	}

	// inner payload
	inner := map[string]string{"hello": "world"}
	innerBytes, err := json.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal inner: %v", err)
	}

	hash := sha256.Sum256(innerBytes)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, hash[:])
	if err != nil {
		t.Fatalf("sign inner: %v", err)
	}

	outer := map[string]interface{}{
		"nonce":     "n-1",
		"data":      json.RawMessage(innerBytes),
		"signature": base64.StdEncoding.EncodeToString(sig),
		"timestamp": int64(1000),
	}
	outerBytes, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("marshal outer: %v", err)
	}

	// call verifyDeviceData using fallback public key
	message, err := verifyDeviceData(1, &pemStr, outerBytes)
	if err != nil {
		t.Fatalf("verifyDeviceData failed: %v", err)
	}
	if message == nil {
		t.Fatalf("verifyDeviceData returned nil message")
	}
}
