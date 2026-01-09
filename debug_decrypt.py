"""
PASTE THIS INTO YOUR EXISTING PYTHON CODE
Add these print statements at the key points in your decrypt function.
"""

# ============================================================================
# OPTION 1: Add these print statements to your existing decrypt function
# ============================================================================

# After you decode the ciphertext from base64:
# print(f"DEBUG_CT_LEN: {len(ciphertext)}")
# print(f"DEBUG_CT_FIRST20: {ciphertext[:20].hex()}")

# After you extract sender_pub_bytes (the 65-byte EC point):
# print(f"DEBUG_SENDER_PUB: {sender_pub_bytes.hex()}")

# After ECDH to get shared_secret:
# print(f"DEBUG_SHARED_SECRET: {shared_secret.hex()}")

# After HKDF to get aes_key:
# print(f"DEBUG_AES_KEY: {aes_key.hex()}")

# Before AES-GCM decrypt - the IV and ciphertext you pass to decrypt:
# print(f"DEBUG_IV: {iv.hex()}")
# print(f"DEBUG_CT_TO_DECRYPT_LEN: {len(ciphertext_to_decrypt)}")


# ============================================================================
# OPTION 2: Wrap your decrypt function with this decorator
# ============================================================================

def debug_decrypt_wrapper(original_decrypt_func):
    """
    Decorator that wraps your decrypt function and prints all values.

    Usage:
        @debug_decrypt_wrapper
        def your_decrypt_function(...):
            ...

    Or:
        your_decrypt_function = debug_decrypt_wrapper(your_decrypt_function)
    """
    def wrapper(*args, **kwargs):
        print("=" * 60)
        print("DECRYPT DEBUG - INPUTS")
        print("=" * 60)
        for i, arg in enumerate(args):
            if isinstance(arg, bytes):
                print(f"  arg[{i}] (bytes, len={len(arg)}): {arg[:50].hex()}...")
            elif isinstance(arg, str) and len(arg) > 100:
                print(f"  arg[{i}] (str, len={len(arg)}): {arg[:50]}...")
            else:
                print(f"  arg[{i}]: {arg}")

        result = original_decrypt_func(*args, **kwargs)

        print("=" * 60)
        print("DECRYPT DEBUG - OUTPUT")
        print("=" * 60)
        if isinstance(result, bytes):
            print(f"  result (bytes, len={len(result)}): {result[:100]}")
        else:
            print(f"  result: {result[:100] if len(str(result)) > 100 else result}")

        return result
    return wrapper


# ============================================================================
# OPTION 3: Complete debug function - paste your decrypt logic inside
# ============================================================================

def debug_full_decrypt(encrypted_token_b64, ephemeral_private_key_d_int, it_metadata=None):
    """
    Complete decrypt with full debug output.

    Paste your decrypt logic inside this function, or call your existing
    functions and add print statements.

    Args:
        encrypted_token_b64: The base64url encoded 'it' value from Google
        ephemeral_private_key_d_int: Your ephemeral private key D as integer
        it_metadata: Optional metadata
    """
    import base64
    import hashlib
    import hmac

    def b64url_decode(s):
        padding = 4 - len(s) % 4
        if padding != 4:
            s += '=' * padding
        return base64.urlsafe_b64decode(s)

    def hkdf_sha256(ikm, salt, info, length):
        if not salt:
            salt = b'\x00' * 32
        prk = hmac.new(salt, ikm, hashlib.sha256).digest()
        okm = b""
        t = b""
        for i in range(1, (length + 31) // 32 + 1):
            t = hmac.new(prk, t + info + bytes([i]), hashlib.sha256).digest()
            okm += t
        return okm[:length]

    # Your imports
    from cryptography.hazmat.primitives.asymmetric import ec
    from cryptography.hazmat.backends import default_backend
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM

    print("=" * 70)
    print("FULL DECRYPT DEBUG")
    print("=" * 70)

    # Step 1: Decode
    ct = b64url_decode(encrypted_token_b64)
    print(f"\n[STEP 1] Base64 decode")
    print(f"  ciphertext length: {len(ct)}")
    print(f"  ciphertext[:20]: {ct[:20].hex()}")

    # Step 2: Parse structure (5-byte prefix + 65-byte pubkey + aes_ct)
    prefix = ct[0:5]
    sender_pub = ct[5:70]
    aes_ct = ct[70:]

    print(f"\n[STEP 2] Parse Tink structure")
    print(f"  prefix[0:5]: {prefix.hex()}")
    print(f"  sender_pub[5:70] (65 bytes): {sender_pub.hex()}")
    print(f"  aes_ct[70:] length: {len(aes_ct)}")
    print(f"  aes_ct[:20]: {aes_ct[:20].hex()}")

    # Step 3: ECDH
    sender_x = int.from_bytes(sender_pub[1:33], 'big')
    sender_y = int.from_bytes(sender_pub[33:65], 'big')

    sender_pubkey = ec.EllipticCurvePublicNumbers(
        sender_x, sender_y, ec.SECP256R1()
    ).public_key(default_backend())

    eph_privkey = ec.derive_private_key(
        ephemeral_private_key_d_int, ec.SECP256R1(), default_backend()
    )

    shared = eph_privkey.exchange(ec.ECDH(), sender_pubkey)

    print(f"\n[STEP 3] ECDH")
    print(f"  sender_x: {sender_x:064x}")
    print(f"  sender_y: {sender_y:064x}")
    print(f"  shared_secret: {shared.hex()}")

    # Step 4: HKDF
    ikm = sender_pub + shared
    aes_key = hkdf_sha256(ikm, b"", b"", 16)

    print(f"\n[STEP 4] HKDF")
    print(f"  IKM (sender_pub || shared): {len(ikm)} bytes")
    print(f"  aes_key: {aes_key.hex()}")

    # Step 5: Try AES-GCM with different structures
    print(f"\n[STEP 5] AES-GCM decrypt attempts")

    aesgcm = AESGCM(aes_key)

    # Try 1: IV at byte 0
    print(f"\n  [Try 1] IV at aes_ct[0:12]")
    iv1 = aes_ct[0:12]
    ct1 = aes_ct[12:]
    print(f"    iv: {iv1.hex()}")
    print(f"    ct+tag length: {len(ct1)}")
    try:
        pt = aesgcm.decrypt(iv1, ct1, None)
        print(f"    SUCCESS! plaintext: {pt[:80]}")
        print(f"\n  WORKING: IV at byte 0, no inner prefix")
        return pt
    except Exception as e:
        print(f"    Failed: {e}")

    # Try 2: Skip 5-byte inner prefix, IV at byte 5
    print(f"\n  [Try 2] IV at aes_ct[5:17] (skip 5-byte inner prefix)")
    inner_prefix = aes_ct[0:5]
    iv2 = aes_ct[5:17]
    ct2 = aes_ct[17:]
    print(f"    inner_prefix: {inner_prefix.hex()}")
    print(f"    iv: {iv2.hex()}")
    print(f"    ct+tag length: {len(ct2)}")
    try:
        pt = aesgcm.decrypt(iv2, ct2, None)
        print(f"    SUCCESS! plaintext: {pt[:80]}")
        print(f"\n  WORKING: IV at byte 5, with 5-byte inner prefix")
        return pt
    except Exception as e:
        print(f"    Failed: {e}")

    print("\n  ALL ATTEMPTS FAILED")
    return None


# ============================================================================
# TEST IT
# ============================================================================

if __name__ == "__main__":
    print("""
To use: In your existing Python code, add this line after getting the token:

    from debug_decrypt import debug_full_decrypt
    result = debug_full_decrypt(it_value, ephemeral_d_as_int)

Or just paste the debug_full_decrypt function into your code and call it.
""")
