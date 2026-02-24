# Documentation

Since the Proton API isn't open sourced, this document serves as the team's understanding for future reference.

# Proton Drive API 

## Terminology

### Volume

### Share

### Node

### Link

## Encryption

Encryption, decryption, and signature signing and verification, etc. are all performed by using the go-crypto library.

### Login

Proton uses SRP for logging in the users. After logging in, there is a small time window (several minutes) where users can access certain routes, which is in the `scope` field, e.g. getting user salt.

Since the user and address key rings are encrypted with passphrase tied to salt and user password, we need to cache this information as soon as the first log in happens for future usage.

### User Key

### Address Key

### Node/Link Key