#!/usr/bin/env python3
"""
Test script to show HKDF values - pure Python implementation.
Run with: python3 test_decrypt.py
"""

import hashlib
import hmac

def hkdf_extract(salt, ikm, hash_algo=hashlib.sha256):
    """HKDF Extract step."""
    if salt is None or len(salt) == 0:
        salt = b'\x00' * hash_algo().digest_size
    return hmac.new(salt, ikm, hash_algo).digest()

def hkdf_expand(prk, info, length, hash_algo=hashlib.sha256):
    """HKDF Expand step."""
    hash_len = hash_algo().digest_size
    n = (length + hash_len - 1) // hash_len
    okm = b""
    t = b""
    for i in range(1, n + 1):
        t = hmac.new(prk, t + info + bytes([i]), hash_algo).digest()
        okm += t
    return okm[:length]

def hkdf(ikm, salt, info, length):
    """Full HKDF."""
    prk = hkdf_extract(salt, ikm)
    return hkdf_expand(prk, info, length)

# Values from Go debug output
shared_secret_hex = "30ad4580d37c8098ac4f09c9e35c146eb44e43ca48ac477e9484a66429a24543"
sender_pub_hex = "046fd9d34a28758d9845787084526df32e49adee71993c64b2af36e3379c609eebc62c5bd58c6c1dc86ee06188e1307ce4222279b7204edfc96171ab7b74359358"

shared_secret = bytes.fromhex(shared_secret_hex)
sender_pub = bytes.fromhex(sender_pub_hex)

print("=== Python HKDF Test ===")
print(f"shared_secret ({len(shared_secret)} bytes): {shared_secret.hex()}")
print(f"sender_pub ({len(sender_pub)} bytes): {sender_pub.hex()}")

# Test 1: IKM = sender_pub || shared_secret, salt=b"", info=b""
print("\n--- Test 1: IKM = sender_pub || shared_secret, salt=b'', info=b'' ---")
ikm1 = sender_pub + shared_secret
key1 = hkdf(ikm1, b"", b"", 16)
print(f"IKM ({len(ikm1)} bytes)")
print(f"AES key: {key1.hex()}")

# Test 2: IKM = shared_secret, salt=sender_pub, info=b""
print("\n--- Test 2: IKM = shared_secret, salt=sender_pub, info=b'' ---")
key2 = hkdf(shared_secret, sender_pub, b"", 16)
print(f"AES key: {key2.hex()}")

# Test 3: IKM = shared_secret, salt=b"", info=b""
print("\n--- Test 3: IKM = shared_secret, salt=b'', info=b'' ---")
key3 = hkdf(shared_secret, b"", b"", 16)
print(f"AES key: {key3.hex()}")

# Test 4: IKM = sender_pub || shared_secret, salt=None (32 zero bytes)
print("\n--- Test 4: IKM = sender_pub || shared_secret, salt=None (zeros) ---")
key4 = hkdf(ikm1, None, b"", 16)
print(f"AES key: {key4.hex()}")

# Go's current output was: aesKey (16 bytes): 6104b3ee2ea3a6b51d5ddf0f5b92f504
print("\n=== Go produced: 6104b3ee2ea3a6b51d5ddf0f5b92f504 (with old IKM=pub||secret, salt=empty) ===")
print("Which test above matches the correct Python output?")
