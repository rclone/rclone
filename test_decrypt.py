#!/usr/bin/env python3
"""
Debug script to show decryption values.
Paste your decrypt function here and call debug_decrypt() with the encrypted token.
"""

import hashlib
import hmac
import base64

def base64url_decode(data):
    """Decode base64url without padding."""
    padding = 4 - len(data) % 4
    if padding != 4:
        data += '=' * padding
    return base64.urlsafe_b64decode(data)

def base64url_encode(data):
    """Encode to base64url without padding."""
    return base64.urlsafe_b64encode(data).rstrip(b'=').decode()

def hkdf_extract(salt, ikm):
    """HKDF Extract step."""
    if salt is None or len(salt) == 0:
        salt = b'\x00' * 32  # SHA256 digest size
    return hmac.new(salt, ikm, hashlib.sha256).digest()

def hkdf_expand(prk, info, length):
    """HKDF Expand step."""
    hash_len = 32
    n = (length + hash_len - 1) // hash_len
    okm = b""
    t = b""
    for i in range(1, n + 1):
        t = hmac.new(prk, t + info + bytes([i]), hashlib.sha256).digest()
        okm += t
    return okm[:length]

def hkdf(ikm, salt, info, length):
    """Full HKDF."""
    prk = hkdf_extract(salt, ikm)
    return hkdf_expand(prk, info, length)

def debug_decrypt(encrypted_token_b64, ephemeral_private_key_hex):
    """
    Debug decryption - shows all intermediate values.

    Args:
        encrypted_token_b64: The base64url encoded encrypted token (from 'it' field in response)
        ephemeral_private_key_hex: The hex string of the ephemeral private key D value
    """
    from cryptography.hazmat.primitives.asymmetric import ec
    from cryptography.hazmat.backends import default_backend
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    print("=== PYTHON DECRYPT DEBUG ===")

    # Decode ciphertext
    ciphertext = base64url_decode(encrypted_token_b64)
    print(f"ciphertext total length: {len(ciphertext)}")
    print(f"ciphertext first 20 bytes: {ciphertext[:20].hex()}")

    # Parse structure
    prefix = ciphertext[0:5]
    print(f"prefix (5 bytes): {prefix.hex()}")

    sender_pub_bytes = ciphertext[5:70]
    print(f"sender_pub_bytes (65 bytes): {sender_pub_bytes.hex()}")
    print(f"sender_pub first byte: {sender_pub_bytes[0]:02x} (should be 04)")

    aes_ciphertext = ciphertext[70:]
    print(f"aes_ciphertext length: {len(aes_ciphertext)}")
    print(f"aes_ciphertext first 20 bytes: {aes_ciphertext[:20].hex()}")

    # Extract sender public key coordinates
    sender_x = int.from_bytes(sender_pub_bytes[1:33], 'big')
    sender_y = int.from_bytes(sender_pub_bytes[33:65], 'big')
    print(f"sender_x: {sender_x:064x}")
    print(f"sender_y: {sender_y:064x}")

    # Load ephemeral private key
    ephemeral_d = int(ephemeral_private_key_hex, 16)
    print(f"ephemeral_private_d: {ephemeral_d:064x}")

    # Reconstruct sender public key
    sender_pub = ec.EllipticCurvePublicNumbers(sender_x, sender_y, ec.SECP256R1()).public_key(default_backend())

    # Create ephemeral private key
    ephemeral_private = ec.derive_private_key(ephemeral_d, ec.SECP256R1(), default_backend())

    # ECDH to get shared secret
    from cryptography.hazmat.primitives import serialization
    shared_key = ephemeral_private.exchange(ec.ECDH(), sender_pub)
    print(f"shared_secret (32 bytes): {shared_key.hex()}")

    # HKDF
    hkdf_ikm = sender_pub_bytes + shared_key
    print(f"hkdf_ikm length: {len(hkdf_ikm)}")
    aes_key = hkdf(hkdf_ikm, b"", b"", 16)
    print(f"aes_key (16 bytes): {aes_key.hex()}")

    # Now try different IV positions
    print("\n=== TRYING DIFFERENT CIPHERTEXT STRUCTURES ===")

    # Structure 1: No inner prefix - IV at byte 0
    print("\n--- Structure 1: IV at byte 0 ---")
    iv1 = aes_ciphertext[0:12]
    ct1 = aes_ciphertext[12:]
    print(f"iv: {iv1.hex()}")
    print(f"ciphertext+tag length: {len(ct1)}")
    try:
        aesgcm = AESGCM(aes_key)
        plaintext = aesgcm.decrypt(iv1, ct1, None)
        print(f"SUCCESS! plaintext: {plaintext[:50]}...")
        return plaintext
    except Exception as e:
        print(f"Failed: {e}")

    # Structure 2: 5-byte inner prefix - IV at byte 5
    print("\n--- Structure 2: IV at byte 5 (skip 5-byte prefix) ---")
    inner_prefix = aes_ciphertext[0:5]
    iv2 = aes_ciphertext[5:17]
    ct2 = aes_ciphertext[17:]
    print(f"inner_prefix: {inner_prefix.hex()}")
    print(f"iv: {iv2.hex()}")
    print(f"ciphertext+tag length: {len(ct2)}")
    try:
        aesgcm = AESGCM(aes_key)
        plaintext = aesgcm.decrypt(iv2, ct2, None)
        print(f"SUCCESS! plaintext: {plaintext[:50]}...")
        return plaintext
    except Exception as e:
        print(f"Failed: {e}")

    # Structure 3: Maybe the outer prefix is different?
    print("\n--- Structure 3: Different outer structure ---")
    # Try with 1-byte version prefix only
    sender_pub_bytes_v2 = ciphertext[1:66]
    aes_ciphertext_v2 = ciphertext[66:]
    print(f"With 1-byte prefix: sender_pub={sender_pub_bytes_v2[:5].hex()}..., aes_ct len={len(aes_ciphertext_v2)}")

    print("\n=== END PYTHON DECRYPT DEBUG ===")
    return None


# If you have the encrypted token and ephemeral key, uncomment and fill in:
# encrypted_token = "YOUR_ENCRYPTED_TOKEN_BASE64URL_HERE"
# ephemeral_key = "YOUR_EPHEMERAL_PRIVATE_KEY_HEX_HERE"
# debug_decrypt(encrypted_token, ephemeral_key)

print("""
To use this script:
1. Add your decrypt code or use the debug_decrypt function above
2. Call it with the encrypted token (base64url) and ephemeral private key (hex)

Example:
    encrypted_token = "AZvRxyz..."  # from Go's 'it' response field
    ephemeral_key = "c5cb0e307f5424..."  # from Go's ephemeralPrivateKey.D output
    debug_decrypt(encrypted_token, ephemeral_key)
""")
