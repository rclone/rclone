package transfer

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"google.golang.org/protobuf/proto"

	hadoop "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_common"
	hdfs "github.com/colinmarc/hdfs/v2/internal/protocol/hadoop_hdfs"
	"github.com/colinmarc/hdfs/v2/internal/sasl"
)

const (
	authMethod    = "TOKEN"
	authMechanism = "DIGEST-MD5"
	authServer    = "0"
	authProtocol  = "hdfs"
)

// SaslDialer dials using the underlying DialFunc, then negotiates
// authentication with the datanode. The resulting Conn implements whatever
// data protection level is specified by the server, whether it be wire
// encryption or integrity checks.
type SaslDialer struct {
	DialFunc                  func(ctx context.Context, network, addr string) (net.Conn, error)
	Key                       *hdfs.DataEncryptionKeyProto
	Token                     *hadoop.TokenProto
	EnforceQop                string
	SkipSaslOnPrivilegedPorts bool
}

func (d *SaslDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.DialFunc == nil {
		d.DialFunc = (&net.Dialer{}).DialContext
	}

	conn, err := d.DialFunc(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// If the port is privileged, and a certain combination of configuration
	// variables are set, hadoop expects us to skip SASL negotiation. See the
	// documentation for ClientOptions in the top-level package for more detail.
	if d.SkipSaslOnPrivilegedPorts {
		if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok && addr.Port < 1024 {
			return conn, nil
		}

	}

	return d.wrapDatanodeConn(conn)
}

// wrapDatanodeConn performs a shortened SASL negotiation with the datanode,
// then returns a wrapped connection or any error encountered. In the case of
// a protection setting of 'authentication', the bare connection is returned.
func (d *SaslDialer) wrapDatanodeConn(conn net.Conn) (net.Conn, error) {
	auth := &hadoop.RpcSaslProto_SaslAuth{}
	auth.Method = proto.String(authMethod)
	auth.Mechanism = proto.String(authMechanism)
	auth.ServerId = proto.String(authServer)
	auth.Protocol = proto.String(authProtocol)

	ourToken := &hadoop.TokenProto{}
	ourToken.Kind = d.Token.Kind
	ourToken.Password = d.Token.Password[:]
	ourToken.Service = d.Token.Service
	ourToken.Identifier = d.Token.GetIdentifier()

	// If the server defaults have EncryptDataTransfer set but the encryption
	// key is empty, the namenode doesn't want us to encrypt the block token.
	if d.Key != nil && len(d.Key.Nonce) > 0 {
		// Amusingly, this is unsigned in the proto struct but is expected
		// to be signed here.
		keyId := int32(d.Key.GetKeyId())

		ourToken.Identifier = []byte(fmt.Sprintf("%d %s %s",
			keyId,
			d.Key.GetBlockPoolId(),
			base64.StdEncoding.EncodeToString(d.Key.Nonce)))
		ourToken.Password = d.Key.EncryptionKey
	} else {
		ourToken.Identifier = make([]byte,
			base64.StdEncoding.EncodedLen(len(d.Token.GetIdentifier())))
		base64.StdEncoding.Encode(ourToken.Identifier, d.Token.GetIdentifier())
	}

	dgst := digestMD5Handshake{
		authID:   ourToken.Identifier,
		passwd:   base64.StdEncoding.EncodeToString(ourToken.Password),
		hostname: auth.GetServerId(),
		service:  auth.GetProtocol(),
	}

	// Begin the handshake with 0xDEADBEEF and an empty message.
	msg := &hdfs.DataTransferEncryptorMessageProto{}
	msg.Status = hdfs.DataTransferEncryptorMessageProto_SUCCESS.Enum()
	data, err := makePrefixedMessage(msg)
	if err != nil {
		return nil, err
	}

	data = append([]byte{0xDE, 0xAD, 0xBE, 0xEF}, data...)
	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	// The response includes a challenge. Compute it and send it back.
	resp := &hdfs.DataTransferEncryptorMessageProto{}
	err = readPrefixedMessage(conn, msg)
	if err != nil {
		return nil, err
	}

	challengeResponse, err := dgst.challengeStep1(msg.Payload)
	if err != nil {
		return nil, err
	}

	// Use the server's QOP unless one was specified in the local configuration.
	privacy := false
	integrity := false
	switch dgst.token.Qop[0] {
	case sasl.QopPrivacy:
		privacy = true
		integrity = true
	case sasl.QopIntegrity:
		if d.EnforceQop == "privacy" {
			return nil, errors.New("negotiating data protection: invalid qop: 'integrity'")
		}

		privacy = false
		integrity = true
	default:
		if d.EnforceQop == "privacy" || d.EnforceQop == "integrity" {
			return nil, fmt.Errorf("negotiating data protection: invalid qop: %s", dgst.token.Qop)
		}
	}

	msg = &hdfs.DataTransferEncryptorMessageProto{}
	msg.Status = hdfs.DataTransferEncryptorMessageProto_SUCCESS.Enum()
	msg.Payload = []byte(challengeResponse)

	if privacy {
		// Indicate to the server that we want AES.
		opt := &hdfs.CipherOptionProto{}
		opt.Suite = hdfs.CipherSuiteProto_AES_CTR_NOPADDING.Enum()
		msg.CipherOption = append(msg.CipherOption, opt)
	}

	data, err = makePrefixedMessage(msg)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	// Read another response from the server.
	err = readPrefixedMessage(conn, resp)
	if err != nil {
		return nil, err
	}

	err = dgst.challengeStep2(resp.Payload)
	if err != nil {
		return nil, err
	}

	// Authentication done; we can return the bare connection if we don't need
	// to do anything else.
	if !privacy && !integrity {
		return conn, nil
	}

	kic, kis := generateIntegrityKeys(dgst.a1())

	var wrapped digestMD5Conn
	if privacy {
		if dgst.cipher == "" {
			return nil, fmt.Errorf("no available cipher among choices: %v", dgst.token.Cipher)
		}

		kcc, kcs := generatePrivacyKeys(dgst.a1(), dgst.cipher)
		wrapped = newDigestMD5PrivacyConn(conn, kic, kis, kcc, kcs)
	} else {
		wrapped = newDigestMD5IntegrityConn(conn, kic, kis)
	}

	// If we're going to encrypt, we use the above wrapped connection just for
	// finishing the handshake.
	if len(resp.GetCipherOption()) > 0 {
		cipher := resp.GetCipherOption()[0]
		var outKey []byte

		decoded, err := wrapped.decode(cipher.InKey)
		if err != nil {
			return nil, err
		}

		inKey := make([]byte, len(decoded))
		copy(inKey, decoded)

		if outKey, err = wrapped.decode(cipher.OutKey); err != nil {
			return nil, err
		}

		return newAesConn(conn, inKey, outKey, cipher.InIv, cipher.OutIv)
	}

	return wrapped, nil
}
