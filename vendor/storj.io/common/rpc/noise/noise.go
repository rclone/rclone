// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package noise

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/x509"
	"encoding/binary"
	"io"
	"time"

	"github.com/flynn/noise"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/blake3"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
)

var (
	mon = monkit.Package()

	// Error is a noise error class.
	Error = errs.Class("noise")
)

// Config is useful for noiseconn Conns.
type Config = noise.Config

// Header is the drpcmigrate.Header prefix for DRPC over Noise.
const Header = "DRPC!N!1"

// DefaultProto is the protobuf enum value that specifies what noise
// protocol should be in use.
const DefaultProto = pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B

// ProtoToConfig takes a pb.NoiseProtocol and returns a noise.Config
// that matches.
func ProtoToConfig(proto pb.NoiseProtocol) (noise.Config, error) {
	switch proto {
	case pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B:
		return noise.Config{
			CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2b),
			Pattern:     noise.HandshakeIK,
		}, nil
	case pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B:
		return noise.Config{
			CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashBLAKE2b),
			Pattern:     noise.HandshakeIK,
		}, nil
	case pb.NoiseProtocol_NOISE_UNSET:
		return noise.Config{}, errs.New("unset noise protocol")
	default:
		return noise.Config{}, errs.New("unknown noise protocol %v", proto)
	}
}

// ConfigToProto is the inverse of ProtoToConfig.
func ConfigToProto(cfg noise.Config) (pb.NoiseProtocol, error) {
	noiseName := cfg.Pattern.Name + "_" + string(cfg.CipherSuite.Name())
	switch noiseName {
	case "IK_25519_ChaChaPoly_BLAKE2b":
		return pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B, nil
	case "IK_25519_AESGCM_BLAKE2b":
		return pb.NoiseProtocol_NOISE_IK_25519_AESGCM_BLAKE2B, nil
	default:
		return pb.NoiseProtocol_NOISE_UNSET, errs.New("unknown noise config %q", noiseName)
	}
}

// ConfigToInfo turns a server-side noise Config into a *pb.NoiseInfo.
func ConfigToInfo(cfg noise.Config) (*pb.NoiseInfo, error) {
	proto, err := ConfigToProto(cfg)
	if err != nil {
		return nil, err
	}
	return &pb.NoiseInfo{
		Proto:     proto,
		PublicKey: cfg.StaticKeypair.Public,
	}, nil
}

func identityBasedEntropy(context string, ident *identity.FullIdentity) (io.Reader, error) {
	h := blake3.NewDeriveKey(context)

	serialized, err := x509.MarshalPKCS8PrivateKey(ident.Key)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = h.Write(serialized)
	return h.Digest(), Error.Wrap(err)
}

// GenerateServerConf makes a server-side noise.Config from a full identity.
func GenerateServerConf(proto pb.NoiseProtocol, ident *identity.FullIdentity) (noise.Config, error) {
	cfg, err := ProtoToConfig(proto)
	if err != nil {
		return noise.Config{}, err
	}
	cfg.Initiator = false

	// we need a server-side keypair, and the way we're going to get it is a bit unusual.
	//
	// first, some context. as discussed in the noise design doc:
	// https://github.com/storj/storj/blob/b022f371d24b64d9435dc02f5cc0de8bf6bff718/docs/blueprints/noise.md
	// it is okay if the server key isn't always stable. we expect the noise key to
	// rotate from time to time, and that a facility exists for clients to get an updated
	// key. in the case of nodes, we expect this rotation to happen commonly enough that
	// node checkins will update the key in the satellite's node table, but infrequently
	// enough that we don't intend to do anything other than close the connection for key
	// mismatch. getting a brand new key every process restart might be fine, but it is
	// a tad uncomfortably often. ultimately, in the ideal case, the key is rotated with the
	// same periodicity as the existing identity leaf key.
	//
	// so, we discussed potentially generating a key and saving it to disk, reusing it if the
	// key is found, which is a nice, natural next step. there are two problems with this
	// approach though:
	//  * when a writeable path is available (and filesystem config is threaded this far
	//    in the call chain), we are writing private key material, and it would be really nice
	//    for the storage node operator or satellite operator or whatever to know and choose
	//    the right permissions. we're trying to avoid operator steps with this rollout, but
	//    making sure the operator sets the right permission here is important. we could do
	//    a umask but ownership matters here too.
	//  * for some instances of this package, we don't always have a writable path at all
	//    (e.g. satellites).
	//
	// thinking laterally, the next idea was to try and use some existing entropy that had the
	// filesystem permissions we wanted, and salt those with some randomness to generate a key
	// that could be saved with world-readable permissions to a temp folder, but have the key
	// require reading files that have permissions operators have already chosen.
	//
	// this is neat, but in the case of app servers for the satellite receiving noise
	// connections (something we intend to do), how do app servers agree on the salt?
	//
	// so that led to here, why have the salt? the leaf private key the node already has is
	// enough entropy that it's not revealable. if we used it as a complete source of
	// entropy, and then made sure to initialize it with context so that you can't go
	// backwards to the key without breaking cryptographic hmac, then we could extend our
	// existing private key into more private keys.
	//
	// so in the below, we generate a blake3 generating reader from the leaf key of the existing
	// key pair, then use that to deterministically generate a noise keypair.
	entropy, err := identityBasedEntropy("storj noise server key", ident)
	if err != nil {
		return noise.Config{}, err
	}
	cfg.StaticKeypair, err = cfg.CipherSuite.GenerateKeypair(entropy)
	if err != nil {
		return noise.Config{}, Error.Wrap(err)
	}
	return cfg, nil
}

// GenerateInitiatorConf makes an initiator noise.Config that talks to the provided peer.
func GenerateInitiatorConf(peer *pb.NoiseInfo) (noise.Config, error) {
	cfg, err := ProtoToConfig(peer.Proto)
	if err != nil {
		return noise.Config{}, err
	}
	keypair, err := cfg.CipherSuite.GenerateKeypair(rand.Reader)
	if err != nil {
		return noise.Config{}, err
	}
	cfg.StaticKeypair = keypair
	cfg.PeerStatic = peer.PublicKey
	cfg.Initiator = true
	return cfg, nil
}

func signablePublicKey(ts time.Time, key []byte) []byte {
	var buf [8]byte
	tsnano := ts.UnixNano()
	if tsnano < 0 {
		tsnano = 0
	}
	binary.BigEndian.PutUint64(buf[:], uint64(tsnano))
	return append(buf[:], key...)
}

// GenerateKeyAttestation will sign a given Noise public key using the
// Node's leaf key and certificate chain, generating a pb.NoiseKeyAttestation.
func GenerateKeyAttestation(ctx context.Context, ident *identity.FullIdentity, info *pb.NoiseInfo) (_ *pb.NoiseKeyAttestation, err error) {
	defer mon.Task()(&ctx)(&err)
	ts := time.Now()
	signature, err := signing.SignerFromFullIdentity(ident).HashAndSign(ctx,
		append([]byte("noise-key-attestation-v1:"), signablePublicKey(ts, info.PublicKey)...))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.NoiseKeyAttestation{
		DeprecatedNodeId: ident.ID,
		NodeCertchain:    identity.EncodePeerIdentity(ident.PeerIdentity()),
		NoiseProto:       info.Proto,
		NoisePublicKey:   info.PublicKey,
		Timestamp:        ts,
		Signature:        signature,
	}, nil
}

// ValidateKeyAttestation will confirm that a provided
// *pb.NoiseKeyAttestation was signed correctly.
func ValidateKeyAttestation(ctx context.Context, attestation *pb.NoiseKeyAttestation, expectedNodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := identity.DecodePeerIdentity(ctx, attestation.NodeCertchain)
	if err != nil {
		return Error.Wrap(err)
	}
	if subtle.ConstantTimeCompare(peer.ID.Bytes(), expectedNodeID.Bytes()) != 1 {
		return Error.New("node id mismatch")
	}
	signee := signing.SigneeFromPeerIdentity(peer)
	unsigned := signablePublicKey(attestation.Timestamp, attestation.NoisePublicKey)
	err = signee.HashAndVerifySignature(ctx,
		append([]byte("noise-key-attestation-v1:"), unsigned...),
		attestation.Signature)
	return Error.Wrap(err)
}

// GenerateSessionAttestation will sign a given Noise session handshake
// hash using the Node's leaf key and certificate chain, generating a
// pb.NoiseSessionAttestation.
func GenerateSessionAttestation(ctx context.Context, ident *identity.FullIdentity, handshakeHash []byte) (_ *pb.NoiseSessionAttestation, err error) {
	defer mon.Task()(&ctx)(&err)
	signature, err := signing.SignerFromFullIdentity(ident).HashAndSign(ctx,
		append([]byte("noise-session-attestation-v1:"), handshakeHash...))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &pb.NoiseSessionAttestation{
		DeprecatedNodeId:   ident.ID,
		NodeCertchain:      identity.EncodePeerIdentity(ident.PeerIdentity()),
		NoiseHandshakeHash: handshakeHash,
		Signature:          signature,
	}, nil
}

// ValidateSessionAttestation will confirm that a provided
// *pb.NoiseSessionAttestation was signed correctly.
func ValidateSessionAttestation(ctx context.Context, attestation *pb.NoiseSessionAttestation, expectedNodeID storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := identity.DecodePeerIdentity(ctx, attestation.NodeCertchain)
	if err != nil {
		return Error.Wrap(err)
	}

	if subtle.ConstantTimeCompare(peer.ID.Bytes(), expectedNodeID.Bytes()) != 1 {
		return Error.New("node id mismatch")
	}
	signee := signing.SigneeFromPeerIdentity(peer)
	err = signee.HashAndVerifySignature(ctx,
		append([]byte("noise-session-attestation-v1:"), attestation.NoiseHandshakeHash...),
		attestation.Signature)
	return Error.Wrap(err)
}
