#!/usr/bin/env python3
"""
Sign MQTT telemetry payloads with ECDSA signature.

Usage:
    python3 sign_mqtt_payload.py [private_key_path]

Example:
    python3 sign_mqtt_payload.py /tmp/device-private.pem
"""
import json
import hashlib
import base64
import time
import sys
from ecdsa import SigningKey, NIST256p, util

def sign_payload(private_key_path, mode, v_rms, temp_c, peak_hz1, peak_hz2, peak_hz3, status):
    """Generate a signed MQTT payload for device telemetry."""
    
    # Load private key
    with open(private_key_path, 'r') as f:
        sk = SigningKey.from_pem(f.read())
    
    # Construct data in canonical order (alphabetically)
    data = {
        "mode": mode,
        "peak_hz_1": int(peak_hz1),
        "peak_hz_2": int(peak_hz2),
        "peak_hz_3": int(peak_hz3),
        "status": status,
        "temp_c": float(temp_c),
        "v_rms": float(v_rms)
    }
    
    # Serialize to JSON with NO SPACES (deterministic)
    data_json = json.dumps(data, separators=(',', ':'), sort_keys=False)
    data_bytes = data_json.encode('utf-8')
    
    # Sign: SHA256(data_bytes) with ECDSA
    data_hash = hashlib.sha256(data_bytes).digest()
    signature_bytes = sk.sign_digest_deterministic(data_hash, hashfunc=hashlib.sha256, sigencode=util.sigencode_der)
    
    # Build envelope with timestamp and nonce
    nonce = f"n-{int(time.time() * 1000)}"
    envelope = {
        "timestamp": int(time.time()),
        "nonce": nonce,
        "data": data,
        "signature": base64.b64encode(signature_bytes).decode('utf-8')
    }
    
    # Return as compact JSON string (must match data serialization)
    return json.dumps(envelope, separators=(',', ':'), sort_keys=False), nonce

if __name__ == '__main__':
    private_key = sys.argv[1] if len(sys.argv) > 1 else '/tmp/device-private.pem'
    
    payload_json, nonce = sign_payload(
        private_key,
        mode="normal",
        v_rms=1.23,
        temp_c=72.4,
        peak_hz1=50,
        peak_hz2=100,
        peak_hz3=150,
        status="ok"
    )
    
    print(payload_json)
