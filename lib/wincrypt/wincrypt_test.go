//go:build windows
// +build windows

package wincrypt

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	reference = []*subjectAttr{
		{
			attrType: COMMONNAME,
			values:   [][]string{{"Test CN1"}, {"Test CN2"}},
		},
		{
			attrType: SERIALNUMBER,
			values:   [][]string{{"Test serial"}},
		},
		{
			attrType: COUNTRYNAME,
			values:   [][]string{{"Test C1", "Test C2"}},
		},
		{
			attrType: LOCALITYNAME,
			values:   [][]string{{"Test L1", "Test L2"}},
		},
		{
			attrType: STATEORPROVINCENAME,
			values:   [][]string{{"Test ST1", "Test ST2"}},
		},
		{
			attrType: STREETADDRESS,
			values:   [][]string{{"Test street1", "Test street2"}},
		},
		{
			attrType: ORGANIZATIONNAME,
			values:   [][]string{{"Test O1", "Test O2"}},
		},
		{
			attrType: ORGANIZATIONALUNITNAME,
			values:   [][]string{{"Test OU1", "Test OU2"}},
		},
		{
			attrType: POSTALCODE,
			values:   [][]string{{"Test postalcode1", "Test postalcode2"}},
		},
	}
)

func TestParseSubject(t *testing.T) {
	subject := "/CN=Test CN1/OU=Test OU1+Test OU2/O=Test O1+Test O2/L=Test L1+Test L2/C=Test C1+Test C2/ST=Test ST1+Test ST2/street=Test street1+Test street2/serialNumber=Test serial/postalCode=Test postalcode1+Test postalcode2/CN=Test CN2"
	subj1, err := parseOpenSSLSubject(subject)
	require.NoError(t, err)
	assert.ElementsMatch(t, reference, subj1)
	subject += "/"
	subj2, err := parseOpenSSLSubject(subject)
	require.NoError(t, err)
	assert.ElementsMatch(t, subj1, subj2)
	subject += "EMAIL=Test Email"
	subj2, err = parseOpenSSLSubject(subject)
	require.NoError(t, err)
	assert.ElementsMatch(t, reference, subj2)
	_, err = parseOpenSSLSubject("/CN=CN=")
	assert.Error(t, err)
}

func TestCertSubjectMatch(t *testing.T) {
	attrs, err := parseOpenSSLSubject("/CN=TestCN/OU=TestOU1+TestOU1")
	require.NoError(t, err)
	cert := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:         "xTestCN1",
			OrganizationalUnit: []string{"xTestOU1", "xTestOU2"},
		},
	}
	assert.False(t, isCertAttributesMatch(cert, attrs))
	cert.Subject.OrganizationalUnit[1] = "xTestOU1"
	assert.True(t, isCertAttributesMatch(cert, attrs))
	attrs, err = parseOpenSSLSubject("/CN=NoMatch")
	require.NoError(t, err)
	assert.False(t, isCertAttributesMatch(cert, attrs))
	attrs, err = parseOpenSSLSubject("/CN=NoMatch/CN=")
	require.NoError(t, err)
	assert.True(t, isCertAttributesMatch(cert, attrs))
	attrs, err = parseOpenSSLSubject("/CN=NoMatch/CN=Test")
	require.NoError(t, err)
	assert.True(t, isCertAttributesMatch(cert, attrs))
	cert = &x509.Certificate{
		Subject: pkix.Name{
			CommonName:         reference[COMMONNAME].values[0][0],
			SerialNumber:       reference[SERIALNUMBER].values[0][0],
			Country:            reference[COUNTRYNAME].values[0],
			Locality:           reference[LOCALITYNAME].values[0],
			Province:           reference[STATEORPROVINCENAME].values[0],
			StreetAddress:      reference[STREETADDRESS].values[0],
			Organization:       reference[ORGANIZATIONNAME].values[0],
			OrganizationalUnit: reference[ORGANIZATIONALUNITNAME].values[0],
			PostalCode:         reference[POSTALCODE].values[0],
		},
	}
	assert.True(t, isCertAttributesMatch(cert, reference))
	subject := "/CN=Test CN2/OU=Test OU1+Test OU2/O=Test O1+Test O2/L=Test L1+Test L2/C=Test C1+Test C2/ST=Test ST1+Test ST2/street=Test street1+Test street2/serialNumber=Test serial/postalCode=Test postalcode1+Test postalcode2"
	attrs, err = parseOpenSSLSubject(subject)
	require.NoError(t, err)
	assert.False(t, isCertAttributesMatch(cert, attrs))
}
