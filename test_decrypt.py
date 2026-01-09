#!/usr/bin/env python3
"""
COMPLETE DEBUG SCRIPT - Run this to show all decryption values.

INSTRUCTIONS:
1. Paste your working decrypt_token() function below (replace the placeholder)
2. Run: python3 test_decrypt.py
3. Share the FULL output with me
"""

import hashlib
import hmac
import base64
import struct

# ============================================================================
# PASTE YOUR WORKING DECRYPT FUNCTION HERE
# Replace the placeholder below with your actual working code
# ============================================================================

def decrypt_token(encrypted_token_b64, ephemeral_private_key, it_metadata=None):
    """
    PLACEHOLDER - Replace this with your working decrypt function.

    Keep all your existing code, just add these print statements at key points:

    1. After decoding ciphertext:
       print(f"DEBUG: ciphertext length: {len(ciphertext)}")
       print(f"DEBUG: ciphertext[:20]: {ciphertext[:20].hex()}")

    2. After extracting sender public key:
       print(f"DEBUG: sender_pub_bytes: {sender_pub_bytes.hex()}")

    3. After ECDH:
       print(f"DEBUG: shared_secret: {shared_secret.hex()}")

    4. After HKDF:
       print(f"DEBUG: aes_key: {aes_key.hex()}")

    5. Before AES-GCM decrypt:
       print(f"DEBUG: iv/nonce: {iv.hex()}")
       print(f"DEBUG: ciphertext_to_decrypt length: {len(ct)}")
       print(f"DEBUG: ciphertext_to_decrypt[:20]: {ct[:20].hex()}")
    """
    raise NotImplementedError("Paste your working decrypt function here!")


# ============================================================================
# ALTERNATIVE: If you can't paste the full function, just fill in these values
# from a working Python run and I'll figure out what's different
# ============================================================================

def manual_debug():
    """
    Fill in these values from your working Python decrypt output.
    Run your Python code with debug prints and paste the values here.
    """

    # From your working Python code, fill in these hex values:
    working_values = {
        # The raw encrypted token (base64url string from Google's response)
        "encrypted_token_b64": "PASTE_THE_IT_FIELD_HERE",

        # Your ephemeral private key D (hex)
        "ephemeral_d_hex": "PASTE_YOUR_PRIVATE_KEY_D_HEX_HERE",

        # From your working decrypt, what are these values?
        "sender_pub_bytes_hex": "PASTE_65_BYTE_SENDER_PUB_HEX",
        "shared_secret_hex": "PASTE_32_BYTE_SHARED_SECRET_HEX",
        "aes_key_hex": "PASTE_16_BYTE_AES_KEY_HEX",
        "iv_nonce_hex": "PASTE_12_BYTE_IV_HEX",

        # CRITICAL: At what byte offset does the IV start in the ciphertext?
        # (after the 5-byte Tink prefix and 65-byte sender pub key)
        # Options: 0 (no inner prefix), 5 (with inner prefix), or other?
        "iv_starts_at_byte": 0,  # or 5, or ?
    }

    return working_values


# ============================================================================
# DEBUG HARNESS - This will trace through decryption step by step
# ============================================================================

def base64url_decode(data):
    padding = 4 - len(data) % 4
    if padding != 4:
        data += '=' * padding
    return base64.urlsafe_b64decode(data)

def hkdf(ikm, salt, info, length):
    """HKDF with SHA256."""
    if salt is None or len(salt) == 0:
        salt = b'\x00' * 32
    prk = hmac.new(salt, ikm, hashlib.sha256).digest()

    # Expand
    okm = b""
    t = b""
    for i in range(1, (length + 31) // 32 + 1):
        t = hmac.new(prk, t + info + bytes([i]), hashlib.sha256).digest()
        okm += t
    return okm[:length]

def full_debug_decrypt(encrypted_token_b64, ephemeral_d_hex):
    """
    Complete debug decryption that tries ALL possible structures.
    """
    try:
        from cryptography.hazmat.primitives.asymmetric import ec
        from cryptography.hazmat.backends import default_backend
        from cryptography.hazmat.primitives.ciphers.aead import AESGCM
        HAS_CRYPTO = True
    except ImportError:
        HAS_CRYPTO = False
        print("WARNING: cryptography module not available, using manual ECDH")

    print("=" * 60)
    print("FULL DECRYPTION DEBUG")
    print("=" * 60)

    # Step 1: Decode ciphertext
    ciphertext = base64url_decode(encrypted_token_b64)
    print(f"\n[1] CIPHERTEXT")
    print(f"    Total length: {len(ciphertext)} bytes")
    print(f"    First 20 bytes: {ciphertext[:20].hex()}")
    print(f"    Last 20 bytes: {ciphertext[-20:].hex()}")

    # Step 2: Parse Tink ECIES structure
    print(f"\n[2] TINK ECIES STRUCTURE")

    # Try different outer prefix sizes
    for prefix_size in [5, 1, 0]:
        print(f"\n    --- Trying outer prefix size: {prefix_size} ---")

        if prefix_size > 0:
            outer_prefix = ciphertext[:prefix_size]
            print(f"    Outer prefix: {outer_prefix.hex()}")

        sender_pub_start = prefix_size
        sender_pub_end = sender_pub_start + 65

        if sender_pub_end > len(ciphertext):
            print(f"    SKIP: Not enough bytes")
            continue

        sender_pub_bytes = ciphertext[sender_pub_start:sender_pub_end]
        print(f"    Sender pub bytes[{sender_pub_start}:{sender_pub_end}]: {sender_pub_bytes[:10].hex()}...")
        print(f"    Sender pub first byte: 0x{sender_pub_bytes[0]:02x} (should be 0x04)")

        if sender_pub_bytes[0] != 0x04:
            print(f"    SKIP: First byte is not 0x04")
            continue

        aes_ciphertext = ciphertext[sender_pub_end:]
        print(f"    AES ciphertext starts at byte {sender_pub_end}")
        print(f"    AES ciphertext length: {len(aes_ciphertext)}")
        print(f"    AES ciphertext first 20: {aes_ciphertext[:20].hex()}")

        # Step 3: ECDH
        print(f"\n[3] ECDH (prefix_size={prefix_size})")
        sender_x = int.from_bytes(sender_pub_bytes[1:33], 'big')
        sender_y = int.from_bytes(sender_pub_bytes[33:65], 'big')
        ephemeral_d = int(ephemeral_d_hex, 16)

        print(f"    Sender X: {sender_x:064x}")
        print(f"    Sender Y: {sender_y:064x}")
        print(f"    Ephemeral D: {ephemeral_d:064x}")

        if HAS_CRYPTO:
            sender_pub = ec.EllipticCurvePublicNumbers(sender_x, sender_y, ec.SECP256R1()).public_key(default_backend())
            ephemeral_private = ec.derive_private_key(ephemeral_d, ec.SECP256R1(), default_backend())
            shared_secret = ephemeral_private.exchange(ec.ECDH(), sender_pub)
        else:
            print("    ERROR: Need cryptography module for ECDH")
            continue

        print(f"    Shared secret: {shared_secret.hex()}")

        # Step 4: HKDF
        print(f"\n[4] HKDF (prefix_size={prefix_size})")
        hkdf_ikm = sender_pub_bytes + shared_secret
        print(f"    IKM = sender_pub || shared_secret ({len(hkdf_ikm)} bytes)")

        aes_key = hkdf(hkdf_ikm, b"", b"", 16)
        print(f"    AES key: {aes_key.hex()}")

        # Step 5: Try different inner structures
        print(f"\n[5] AES-GCM DECRYPT ATTEMPTS (prefix_size={prefix_size})")

        for inner_prefix_size in [0, 5]:
            print(f"\n    --- Inner prefix size: {inner_prefix_size} ---")

            if inner_prefix_size > 0:
                inner_prefix = aes_ciphertext[:inner_prefix_size]
                print(f"    Inner prefix: {inner_prefix.hex()}")

            iv_start = inner_prefix_size
            iv_end = iv_start + 12

            if iv_end > len(aes_ciphertext):
                print(f"    SKIP: Not enough bytes for IV")
                continue

            iv = aes_ciphertext[iv_start:iv_end]
            ct_with_tag = aes_ciphertext[iv_end:]

            print(f"    IV [{iv_start}:{iv_end}]: {iv.hex()}")
            print(f"    Ciphertext+tag length: {len(ct_with_tag)}")
            print(f"    Ciphertext+tag first 20: {ct_with_tag[:min(20, len(ct_with_tag))].hex()}")

            if HAS_CRYPTO:
                try:
                    aesgcm = AESGCM(aes_key)
                    plaintext = aesgcm.decrypt(iv, ct_with_tag, None)
                    print(f"    *** SUCCESS! ***")
                    print(f"    Plaintext length: {len(plaintext)}")
                    print(f"    Plaintext (first 100): {plaintext[:100]}")
                    print(f"\n    WORKING STRUCTURE:")
                    print(f"      Outer prefix size: {prefix_size}")
                    print(f"      Inner prefix size: {inner_prefix_size}")
                    print(f"      IV at byte: {sender_pub_end + inner_prefix_size}")
                    return plaintext
                except Exception as e:
                    print(f"    Failed: {e}")

    print("\n" + "=" * 60)
    print("ALL ATTEMPTS FAILED")
    print("=" * 60)
    return None


# ============================================================================
# MAIN - Run this
# ============================================================================

if __name__ == "__main__":
    print("""
============================================================
GOOGLE PHOTOS TOKEN DECRYPTION DEBUG SCRIPT
============================================================

To use this script, you need TWO values from Go's debug output:

1. The encrypted token (from Google's response 'it' field)
   - This is a base64url string

2. The ephemeral private key D (hex)
   - From Go output: "ephemeral private D: XXXX"

Run rclone once to get these values, then paste them below.
============================================================
""")

    # PASTE YOUR VALUES HERE:
    encrypted_token = """PASTE_ENCRYPTED_TOKEN_HERE"""
    ephemeral_d = """PASTE_EPHEMERAL_D_HEX_HERE"""

    # Remove whitespace
    encrypted_token = encrypted_token.strip()
    ephemeral_d = ephemeral_d.strip()

    if "PASTE" in encrypted_token or "PASTE" in ephemeral_d:
        print("Please edit this file and paste your values!")
        print("\nAlternatively, call the function directly:")
        print('  full_debug_decrypt("your_token_b64", "your_key_hex")')
    else:
        full_debug_decrypt(encrypted_token, ephemeral_d)
