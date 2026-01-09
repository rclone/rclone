#!/usr/bin/env python3
"""Verify ECDH computation matches Go's output."""

# P-256 curve parameters
P = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffff
A = 0xffffffff00000001000000000000000000000000fffffffffffffffffffffffc
N = 0xffffffff00000000ffffffffffffffffbce6faada7179e84f3b9cac2fc632551

def mod_inverse(k, p):
    return pow(k, p - 2, p)

def point_add(P1, P2):
    if P1 is None:
        return P2
    if P2 is None:
        return P1
    x1, y1 = P1
    x2, y2 = P2
    if x1 == x2 and y1 != y2:
        return None
    if x1 == x2:
        m = (3 * x1 * x1 + A) * mod_inverse(2 * y1, P) % P
    else:
        m = (y2 - y1) * mod_inverse(x2 - x1, P) % P
    x3 = (m * m - x1 - x2) % P
    y3 = (m * (x1 - x3) - y1) % P
    return (x3, y3)

def scalar_mult(k, point):
    result = None
    addend = point
    while k:
        if k & 1:
            result = point_add(result, addend)
        addend = point_add(addend, addend)
        k >>= 1
    return result

# From Go's debug output
sender_x = int("a809751c27e5ee275cbc8f3ce35bc36ee68b01ce90ce234cfbcb5e0b2d631adb", 16)
sender_y = int("7eb0a2fb263cc97c8c619389757ee81f9969bcef1e1ec0f9c9dd8ef6c1bfc24b", 16)
our_d = int("eaff178d418c6fc9a66c2255971c62eeba9b056ebe65379b97b120e7d4975bbf", 16)

print("ECDH Verification")
print("="*60)
print(f"Sender X: {sender_x:064x}")
print(f"Sender Y: {sender_y:064x}")
print(f"Our D:    {our_d:064x}")

# Compute ECDH: sender_pub * our_d
result_point = scalar_mult(our_d, (sender_x, sender_y))
shared_x, shared_y = result_point

print(f"\nECDH Result X (shared secret): {shared_x:064x}")
print(f"ECDH Result Y:                 {shared_y:064x}")

# Go reported shared secret
go_shared = "0e549c5b9730040ee8e85e45f72131f61a1aeff455b563db7807756098add0ff"
print(f"\nGo's shared secret:            {go_shared}")

# Compare
python_shared = f"{shared_x:064x}"
print(f"Python's shared secret:        {python_shared}")
print(f"\nMatch: {python_shared == go_shared}")

# Also show what aesCiphertext starts with
aes_ct_start = "0e549c5b9730040ee8e85e45f72131f61a1aeff4"
print(f"\naesCiphertext[0:20]:           {aes_ct_start}")
print(f"sharedSecret[0:20]:            {go_shared[:40]}")
print(f"\nThey match: {aes_ct_start == go_shared[:40]}")

print("\n" + "="*60)
print("CONCLUSION:")
print("="*60)
print("""
The fact that aesCiphertext starts with sharedSecret bytes is
IMPOSSIBLE unless:
1. The ciphertext structure is completely different than expected
2. There's an encoding issue in parsing
3. The ECIES implementation uses a non-standard format

Let's check the actual ciphertext bytes...
""")

import base64

def b64url_decode(s):
    padding = 4 - len(s) % 4
    if padding != 4:
        s += '=' * padding
    return base64.urlsafe_b64decode(s)

it_value = "ASAwxy4EqAl1HCfl7idcvI8841vDbuaLAc6QziNM-8teCy1jGtt-sKL7JjzJfIxhk4l1fugfmWm87x4ewPnJ3Y72wb_CS8vjr5JZPSmwoVd7SB9_50OgcTJgHbgRyoMwobP5grar6khj2nV5uRRkinbzTRe0bX4Q1KM4F0A_wr0L20QgHBgfYsXkgwWgPGKuXx_RpZEo_uV46a11NvfXDNsoPssvqA75vNTESJ539UD_ueDRcQ6bO5xkNQTnBI-KskESctxWAxHo3BFPLYfgny7ZW-l8NHmyKdPDutVM5tiqP-UWJizAhDSP24Oafl_o4Bqezi8xaSsts1uMAL-ujBA64ufa5cyj0LIh4Uz7FwXOmyLChjsMg6P8AzEaMc88tmXU5FJI5NjjyIA2CTu9MXL1zq121EBua4C4IAMYFNB3XNUg9AWgDVRalJBJwkUggwpadAygqX1yiq8URwbzUvJkyJMEYfbuBRJDLwLwpj9oR6bkzNLGTkYXsBT_Kjbm-8PFKWfzev4Dn-KRFZ2XgkZFyNx4aNN3vlomM_N-9yglE5KbLHAz8AFfCeM8QKrksRYBJqJIe5uCf4Pwk6J-h7l68OjUDta8Xqpyh6x47yWQ1roemAZKwvklzfPGBYrWKaKCkbWXR9xoj1itZYRPSx5mFV9gkIKgawW2qvXCX41BZGowX1O9t4FbqNQh0weweWZ50hrZu_GxXL4Z7Rtdf8kk_Puc8zmB6usMLJIUnot18kPQgpyz1TFFvMu7BRt65vS7GEa7IpP8ujPWbNIryesynFnaDq-eU1blAG_0lhNQ7RrE"

ct = b64url_decode(it_value)
print(f"Total ciphertext length: {len(ct)}")
print(f"\nHex dump of first 100 bytes:")
print(ct[:100].hex())

print(f"\nByte 0-4 (prefix):      {ct[0:5].hex()}")
print(f"Byte 5 (EC marker):     {ct[5]:02x}")
print(f"Byte 5-69 (sender pub): {ct[5:70].hex()}")
print(f"Byte 70-89 (aes_ct):    {ct[70:90].hex()}")
print(f"Byte 70-89 == shared[0:20]: {ct[70:90].hex() == go_shared[:40]}")
