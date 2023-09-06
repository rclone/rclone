package spnego

import (
	"encoding/asn1"

	"github.com/geoffgarside/ber"
)

var (
	SpnegoOid     = asn1.ObjectIdentifier([]int{1, 3, 6, 1, 5, 5, 2})
	MsKerberosOid = asn1.ObjectIdentifier([]int{1, 2, 840, 48018, 1, 2, 2})
	KerberosOid   = asn1.ObjectIdentifier([]int{1, 2, 840, 113554, 1, 2, 2})
	NlmpOid       = asn1.ObjectIdentifier([]int{1, 3, 6, 1, 4, 1, 311, 2, 2, 10})
)

type initialContextToken struct { // `asn1:"application,tag:0"`
	ThisMech asn1.ObjectIdentifier `asn1:"optional"`
	Init     []NegTokenInit        `asn1:"optional,explict,tag:0"`
	Resp     []NegTokenResp        `asn1:"optional,explict,tag:1"`
}

type initialContextToken2 struct { // `asn1:"application,tag:0"`
	ThisMech asn1.ObjectIdentifier `asn1:"optional"`
	Init2    []NegTokenInit2       `asn1:"optional,explict,tag:0"`
	Resp     []NegTokenResp        `asn1:"optional,explict,tag:1"`
}

// initialContextToken ::= [APPLICATION 0] IMPLICIT SEQUENCE {
//   ThisMech          MechType
//   InnerContextToken negotiateToken
// }

// negotiateToken ::= CHOICE {
//   NegTokenInit [0] NegTokenInit
//   NegTokenResp [1] NegTokenResp
// }

type NegTokenInit struct {
	MechTypes   []asn1.ObjectIdentifier `asn1:"explicit,optional,tag:0"`
	ReqFlags    asn1.BitString          `asn1:"explicit,optional,tag:1"`
	MechToken   []byte                  `asn1:"explicit,optional,tag:2"`
	MechListMIC []byte                  `asn1:"explicit,optional,tag:3"`
}

// "not_defined_in_RFC4178@please_ignore"
var negHints = asn1.RawValue{
	FullBytes: []byte{
		0xa3, 0x2a, 0x30, 0x28, 0xa0, 0x26, 0x1b, 0x24, 0x6e, 0x6f, 0x74, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x65, 0x64, 0x5f,
		0x69, 0x6e, 0x5f, 0x52, 0x46, 0x43, 0x34, 0x31, 0x37, 0x38, 0x40, 0x70, 0x6c, 0x65, 0x61, 0x73,
		0x65, 0x5f, 0x69, 0x67, 0x6e, 0x6f, 0x72, 0x65,
	},
}

// type NegHint struct {
// HintName    string `asn1:"optional,explicit,tag:0"` // GeneralString = 27
// HintAddress []byte `asn1:"optional,explicit,tag:1"`
// }

type NegTokenInit2 struct {
	MechTypes   []asn1.ObjectIdentifier `asn1:"explicit,optional,tag:0"`
	ReqFlags    asn1.BitString          `asn1:"explicit,optional,tag:1"`
	MechToken   []byte                  `asn1:"explicit,optional,tag:2"`
	NegHints    asn1.RawValue           `asn1:"explicit,optional,tag:3"`
	MechListMIC []byte                  `asn1:"explicit,optional,tag:4"`
}

type NegTokenResp struct {
	NegState      asn1.Enumerated       `asn1:"optional,explicit,tag:0"`
	SupportedMech asn1.ObjectIdentifier `asn1:"optional,explicit,tag:1"`
	ResponseToken []byte                `asn1:"optional,explicit,tag:2"`
	MechListMIC   []byte                `asn1:"optional,explicit,tag:3"`
}

func DecodeNegTokenInit2(bs []byte) (*NegTokenInit2, error) {
	var init initialContextToken2

	_, err := ber.UnmarshalWithParams(bs, &init, "application,tag:0")
	if err != nil {
		return nil, err
	}

	return &init.Init2[0], nil
}

func EncodeNegTokenInit2(types []asn1.ObjectIdentifier) ([]byte, error) {
	bs, err := asn1.Marshal(
		initialContextToken2{
			ThisMech: SpnegoOid,
			Init2: []NegTokenInit2{
				{
					MechTypes: types,
					NegHints:  negHints,
				},
			},
		})
	if err != nil {
		return nil, err
	}

	bs[0] = 0x60 // `asn1:"application,tag:0"`

	return bs, nil
}

func EncodeNegTokenInit(types []asn1.ObjectIdentifier, token []byte) ([]byte, error) {
	bs, err := asn1.Marshal(
		initialContextToken{
			ThisMech: SpnegoOid,
			Init: []NegTokenInit{
				{
					MechTypes: types,
					MechToken: token,
				},
			},
		})
	if err != nil {
		return nil, err
	}

	bs[0] = 0x60 // `asn1:"application,tag:0"`

	return bs, nil
}

func DecodeNegTokenInit(bs []byte) (*NegTokenInit, error) {
	var init initialContextToken

	_, err := ber.UnmarshalWithParams(bs, &init, "application,tag:0")
	if err != nil {
		return nil, err
	}

	return &init.Init[0], nil
}

func EncodeNegTokenResp(state asn1.Enumerated, typ asn1.ObjectIdentifier, token, mechListMIC []byte) ([]byte, error) {
	bs, err := asn1.Marshal(
		initialContextToken{
			Resp: []NegTokenResp{
				{
					NegState:      state,
					SupportedMech: typ,
					ResponseToken: token,
					MechListMIC:   mechListMIC,
				},
			},
		})
	if err != nil {
		return nil, err
	}

	skip := 1
	if bs[skip] < 128 {
		skip += 1
	} else {
		skip += int(bs[skip]) - 128 + 1
	}

	return bs[skip:], nil
}

func DecodeNegTokenResp(bs []byte) (*NegTokenResp, error) {
	var resp NegTokenResp

	_, err := ber.UnmarshalWithParams(bs, &resp, "explicit,tag:1")
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// real world example

// structured:
// 0x00: non-structured
// 0x20: structured

// class:
// 0x00: general
// 0x40: application
// 0x80: context specific
// 0xc0: private

// 0x60 = 0x20 | 0x40
// 0xa0 = 0x20 | 0x80

// Negotiate Message
// 60 48 (initialContextToken)
//    06 06 (type)
//       2b 06 01 05 05 02
//    a0 3e (tag 0)
//       30 3c (negTokenInit)
//          a0 0e (tag 0)
//             30 0c (mechTypes)
//                06 0a (mechType)
//                   2b 06 01 04 01 82 37 02 02 0a
//          a2 2a (tag 2)
//             04 28 (mechToken)
//                4e 54 4c 4d 53 53 50 00 01 00 00 00 97 82 08 e2 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 0a 00 5a 29 00 00 00 0f

// Challenge Message
//    a1 81 ca  (tag 1)
//       30 81 c7 (negTokenResp)
//          a0 03 (tag 0)
//             0a 01 (negState)
//                01
//          a1 0c (tag 1)
//             06 0a (supportedMech)
//                2b 06 01 04 01 82 37 02 02 0a
//          a2 81 b1 (tag 2)
//             04 81 ae (responseToken)
//                4e 54 4c 4d 53 53 50 00 02 00 00 00 10 00 10 00 38 00 00 00 35 82 89 62 a9 d9 c9 2c f4 15 2e 98 00 00 00 00 00 00 00 00 66 00 66 00 48 00 00 00 06 01 b0 1d 0f 00 00 00 46 00 41 00 4b 00 45 00 52 00 55 00 4e 00 45 00 01 00 10 00 46 00 41 00 4b 00 45 00 52 00 55 00 4e 00 45 00 02 00 10 00 46 00 41 00 4b 00 45 00 52 00 55 00 4e 00 45 00 03 00 1c 00 66 00 61 00 6b 00 65 00 72 00 75 00 6e 00 65 00 2e 00 6c 00 6f 00 63 00 61 00 6c 00 04 00 0a 00 6c 00 6f 00 63 00 61 00 6c 00 07 00 08 00 00 76 b9 15 16 c2 d1 01 00 00 00 00

// Authenticate Message
//    a1 82 02 07 (tag 1)
//       30 82 02 03 (negTokenResp)
//          a0 03 (tag 0)
//             0a 01 (negState)
//                01
//          a2 82 01 e6 (tag 2)
//             04 82 01 e2  (responseToken)
//                4e 54 4c 4d 53 53 50 00 03 00 00 00 18 00 18 00 ac 00 00 00 0e 01 0e 01 c4 00 00 00 20 00 20 00 58 00 00 00 26 00 26 00 78 00 00 00 0e 00 0e 00 9e 00 00 00 10 00 10 00 d2 01 00 00 15 82 88 62 0a 00 5a 29 00 00 00 0f 3e 3d 42 66 11 05 d1 43 9d ee 00 f8 36 ca d4 fa 4d 00 69 00 63 00 72 00 6f 00 73 00 6f 00 66 00 74 00 41 00 63 00 63 00 6f 00 75 00 6e 00 74 00 68 00 69 00 72 00 6f 00 65 00 69 00 6b 00 6f 00 40 00 6f 00 75 00 74 00 6c 00 6f 00 6f 00 6b 00 2e 00 6a 00 70 00 48 00 4f 00 4d 00 45 00 2d 00 50 00 43 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 bf 30 2e 94 f7 61 de 33 28 8f 11 86 6a 37 b2 9c 01 01 00 00 00 00 00 00 00 76 b9 15 16 c2 d1 01 27 53 c1 0d 33 3a 7b 10 00 00 00 00 01 00 10 00 46 00 41 00 4b 00 45 00 52 00 55 00 4e 00 45 00 02 00 10 00 46 00 41 00 4b 00 45 00 52 00 55 00 4e 00 45 00 03 00 1c 00 66 00 61 00 6b 00 65 00 72 00 75 00 6e 00 65 00 2e 00 6c 00 6f 00 63 00 61 00 6c 00 04 00 0a 00 6c 00 6f 00 63 00 61 00 6c 00 07 00 08 00 00 76 b9 15 16 c2 d1 01 06 00 04 00 02 00 00 00 08 00 30 00 30 00 00 00 00 00 00 00 01 00 00 00 00 20 00 00 05 2b 42 bd 2c fd f1 05 bc 03 8d e9 3d 80 37 5c 47 f4 33 66 bb 93 76 57 9c f2 e7 ff cf d0 6a af 0a 00 10 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 09 00 20 00 63 00 69 00 66 00 73 00 2f 00 31 00 39 00 32 00 2e 00 31 00 36 00 38 00 2e 00 30 00 2e 00 37 00 00 00 00 00 00 00 00 00 00 00 00 00 84 9e e9 fc d7 0e a9 2c 0c 4f 60 e0 df aa f6 d2
//          a3 12 (tag 3)
//             04 10 (mechList MIC)
//                01 00 00 00 69 e2 49 81 b5 da c3 3f 00 00 00 00
