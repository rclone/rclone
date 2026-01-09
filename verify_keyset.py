#!/usr/bin/env python3
"""Decode and verify the Tink keyset encoding - no external crypto libs."""

import base64

def b64url_decode(s):
    padding = 4 - len(s) % 4
    if padding != 4:
        s += '=' * padding
    return base64.urlsafe_b64decode(s)

def parse_varint(data, pos):
    result = 0
    shift = 0
    while pos < len(data):
        b = data[pos]
        pos += 1
        result |= (b & 0x7F) << shift
        if (b & 0x80) == 0:
            break
        shift += 7
    return result, pos

def parse_proto(data):
    fields = {}
    pos = 0
    while pos < len(data):
        if pos >= len(data):
            break
        tag, pos = parse_varint(data, pos)
        field_num = tag >> 3
        wire_type = tag & 0x7
        if wire_type == 0:
            value, pos = parse_varint(data, pos)
            fields[field_num] = ('varint', value)
        elif wire_type == 2:
            length, pos = parse_varint(data, pos)
            if pos + length > len(data):
                break
            value = data[pos:pos+length]
            pos += length
            fields[field_num] = ('bytes', value)
        else:
            break
    return fields

# P-256 curve parameters
P = 0xffffffff00000001000000000000000000000000ffffffffffffffffffffffff
A = 0xffffffff00000001000000000000000000000000fffffffffffffffffffffffc
B = 0x5ac635d8aa3a93e7b3ebbd55769886bc651d06b0cc53b0f63bce3c3e27d2604b
Gx = 0x6b17d1f2e12c4247f8bce6e563a440f277037d812deb33a0f4a13945d898c296
Gy = 0x4fe342e2fe1a7f9b8ee7eb4a7c0f9e162bce33576b315ececbb6406837bf51f5
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

# Parse keyset
keyset_b64 = "CK6Ow4ECEt0BCtABCj10eXBlLmdvb2dsZWFwaXMuY29tL2dvb2dsZS5jcnlwdG8udGluay5FY2llc0FlYWRIa2RmUHVibGljS2V5EowBEkQKBAgCEAMSOhI4CjB0eXBlLmdvb2dsZWFwaXMuY29tL2dvb2dsZS5jcnlwdG8udGluay5BZXNHY21LZXkSAhAQGAEYARohAGlyLlKZQVQezIdOb2IT5-MsDbUOJyTrTkwadsx1AmlVIiEAJrLSr4FhBnUTUI3MQQKK8YVi1xE-FqjCEhuTQVg2WcoYAxABGK6Ow4ECIAE"

keyset_bytes = b64url_decode(keyset_b64)
keyset = parse_proto(keyset_bytes)
key_data = keyset[2][1]
key = parse_proto(key_data)
kd = parse_proto(key[1][1])
pub_key_value = kd[2][1]
pk = parse_proto(pub_key_value)

x_from_keyset = pk[3][1]
y_from_keyset = pk[4][1]

print("="*60)
print("KEYSET CONTAINS:")
print("="*60)
print(f"X ({len(x_from_keyset)} bytes): {x_from_keyset.hex()}")
print(f"Y ({len(y_from_keyset)} bytes): {y_from_keyset.hex()}")

# Strip leading 00 if present
if x_from_keyset[0] == 0:
    x_from_keyset = x_from_keyset[1:]
if y_from_keyset[0] == 0:
    y_from_keyset = y_from_keyset[1:]

keyset_x_int = int.from_bytes(x_from_keyset, 'big')
keyset_y_int = int.from_bytes(y_from_keyset, 'big')

print(f"\nKeyset X (int): {keyset_x_int:064x}")
print(f"Keyset Y (int): {keyset_y_int:064x}")

# Compute expected public key from private D
print("\n" + "="*60)
print("COMPUTING EXPECTED PUBLIC KEY FROM PRIVATE D")
print("="*60)

d_hex = "eaff178d418c6fc9a66c2255971c62eeba9b056ebe65379b97b120e7d4975bbf"
d_int = int(d_hex, 16)

expected_point = scalar_mult(d_int, (Gx, Gy))
expected_x, expected_y = expected_point

print(f"Expected X: {expected_x:064x}")
print(f"Expected Y: {expected_y:064x}")

print("\n" + "="*60)
print("COMPARISON")
print("="*60)

x_match = keyset_x_int == expected_x
y_match = keyset_y_int == expected_y

print(f"X matches: {x_match}")
print(f"Y matches: {y_match}")

if not x_match:
    print(f"\n  Keyset X:  {keyset_x_int:064x}")
    print(f"  Expected X: {expected_x:064x}")

if not y_match:
    print(f"\n  Keyset Y:  {keyset_y_int:064x}")
    print(f"  Expected Y: {expected_y:064x}")

if x_match and y_match:
    print("\n*** KEYSET ENCODING IS CORRECT! ***")
    print("Google received the right public key.")
    print("The problem must be in how we're parsing the encrypted response.")
else:
    print("\n*** KEYSET ENCODING IS WRONG! ***")
    print("Google is receiving different coordinates than we computed!")
