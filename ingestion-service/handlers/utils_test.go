package handlers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"ingestion-service/db"
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

func TestVerifyRawDataStructure_Valid(t *testing.T) {
	payload := map[string]interface{}{
		"timestamp": int64(1000),
		"nonce":     "abc",
		"data":      json.RawMessage(`{"x":1}`),
		"signature": "sig",
	}
	b, _ := json.Marshal(payload)
	msg, err := verifyRawDataStructure(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil message")
	}
}

func TestVerifyRawDataStructure_MissingFields(t *testing.T) {
	cases := []struct {
		name    string
		payload string
	}{
		{"missing_data", `{"timestamp":1000,"nonce":"abc","signature":"sig"}`},
		{"missing_nonce", `{"timestamp":1000,"data":{"x":1},"signature":"sig"}`},
		{"missing_signature", `{"timestamp":1000,"nonce":"abc","data":{"x":1}}`},
		{"missing_timestamp", `{"nonce":"abc","data":{"x":1},"signature":"sig"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifyRawDataStructure([]byte(tc.payload)); err == nil {
				t.Errorf("expected error for payload: %s", tc.payload)
			}
		})
	}
}

func TestVerifyRawDataStructure_InvalidJSON(t *testing.T) {
	if _, err := verifyRawDataStructure([]byte("not-json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePublicKey_EmptyPEM(t *testing.T) {
	if _, err := ParsePublicKey(""); err == nil {
		t.Fatal("expected error for empty PEM string")
	}
}

func TestParsePublicKey_NonECDSAKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal RSA public key: %v", err)
	}
	pemStr := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	if _, err := ParsePublicKey(pemStr); err == nil {
		t.Fatal("expected error for non-ECDSA key")
	}
}

func TestVerifySignature_BadSignature(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	payload := []byte("data")
	badSig := []byte("this-is-not-a-valid-signature")
	if verifySignature(&priv.PublicKey, payload, badSig) {
		t.Fatal("expected verifySignature to return false for a bad signature")
	}
}

func TestVerifySignature_WrongKey(t *testing.T) {
	priv1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	priv2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	payload := []byte("data")
	hash := sha256.Sum256(payload)
	sig, _ := ecdsa.SignASN1(rand.Reader, priv1, hash[:])
	if verifySignature(&priv2.PublicKey, payload, sig) {
		t.Fatal("expected verifySignature to return false when signed with a different key")
	}
}

func TestPrepareKafkaPayload_FieldMapping(t *testing.T) {
	data := db.SensorDeviceData{
		Mode:    "normal",
		VRMS:    1.5,
		TempC:   36.6,
		PeakHz1: 10,
		PeakHz2: 20,
		PeakHz3: 30,
		Status:  "ok",
	}
	kp := prepareKafkaPayload(99, 1234, data)

	checks := map[string]bool{
		"device_id":  kp.DeviceID == 99,
		"timestamp":  kp.Timestamp == 1234,
		"mode":       kp.Mode == data.Mode,
		"vrms":       kp.VRMS == data.VRMS,
		"temp_c":     kp.TempC == data.TempC,
		"peak_hz_1":  kp.PeakHz1 == data.PeakHz1,
		"peak_hz_2":  kp.PeakHz2 == data.PeakHz2,
		"peak_hz_3":  kp.PeakHz3 == data.PeakHz3,
		"status":     kp.Status == data.Status,
	}
	for field, ok := range checks {
		if !ok {
			t.Errorf("KafkaPayload.%s not set correctly", field)
		}
	}
}

func TestVerifyDeviceData_NilFallback(t *testing.T) {
	payload := map[string]interface{}{
		"timestamp": int64(1000),
		"nonce":     "n1",
		"data":      json.RawMessage(`{"x":1}`),
		"signature": base64.StdEncoding.EncodeToString([]byte("fake")),
	}
	b, _ := json.Marshal(payload)
	_, err := verifyDeviceData(1, nil, b)
	if err == nil {
		t.Fatal("expected error when fallback public key is nil")
	}
}

func TestVerifyDeviceData_InvalidSignature(t *testing.T) {
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pemStr, _ := pemFromPublicKey(&priv.PublicKey)

	inner := json.RawMessage(`{"x":1}`)
	// sign with a different key so verification fails
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	hash := sha256.Sum256(inner)
	sig, _ := ecdsa.SignASN1(rand.Reader, other, hash[:])

	payload := map[string]interface{}{
		"timestamp": int64(1000),
		"nonce":     "n2",
		"data":      inner,
		"signature": base64.StdEncoding.EncodeToString(sig),
	}
	b, _ := json.Marshal(payload)
	_, err := verifyDeviceData(2, &pemStr, b)
	if err == nil {
		t.Fatal("expected error for invalid signature")
	}
}
