package smb2

import (
	"encoding/asn1"

	"github.com/hirochachacha/go-smb2/internal/spnego"
)

type spnegoClient struct {
	mechs        []Initiator
	mechTypes    []asn1.ObjectIdentifier
	selectedMech Initiator
}

func newSpnegoClient(mechs []Initiator) *spnegoClient {
	mechTypes := make([]asn1.ObjectIdentifier, len(mechs))
	for i, mech := range mechs {
		mechTypes[i] = mech.oid()
	}
	return &spnegoClient{
		mechs:     mechs,
		mechTypes: mechTypes,
	}
}

func (c *spnegoClient) oid() asn1.ObjectIdentifier {
	return spnego.SpnegoOid
}

func (c *spnegoClient) initSecContext() (negTokenInitBytes []byte, err error) {
	mechToken, err := c.mechs[0].initSecContext()
	if err != nil {
		return nil, err
	}
	negTokenInitBytes, err = spnego.EncodeNegTokenInit(c.mechTypes, mechToken)
	if err != nil {
		return nil, err
	}
	return negTokenInitBytes, nil
}

func (c *spnegoClient) acceptSecContext(negTokenRespBytes []byte) (negTokenRespBytes1 []byte, err error) {
	negTokenResp, err := spnego.DecodeNegTokenResp(negTokenRespBytes)
	if err != nil {
		return nil, err
	}

	for i, mechType := range c.mechTypes {
		if mechType.Equal(negTokenResp.SupportedMech) {
			c.selectedMech = c.mechs[i]
			break
		}
	}

	responseToken, err := c.selectedMech.acceptSecContext(negTokenResp.ResponseToken)
	if err != nil {
		return nil, err
	}

	ms, err := asn1.Marshal(c.mechTypes)
	if err != nil {
		return nil, err
	}

	mechListMIC := c.selectedMech.sum(ms)

	negTokenRespBytes1, err = spnego.EncodeNegTokenResp(1, nil, responseToken, mechListMIC)
	if err != nil {
		return nil, err
	}

	return negTokenRespBytes1, nil
}

func (c *spnegoClient) sum(bs []byte) []byte {
	return c.selectedMech.sum(bs)
}

func (c *spnegoClient) sessionKey() []byte {
	return c.selectedMech.sessionKey()
}
