package proton

type AuthInfoReq struct {
	Username string
}

type AuthInfo struct {
	Version         int
	Modulus         string
	ServerEphemeral string
	Salt            string
	SRPSession      string
	TwoFA           TwoFAInfo `json:"2FA"`
}

type AuthVerifier struct {
	Version   int
	ModulusID string
	Salt      string
	Verifier  string
}

type AuthModulus struct {
	Modulus   string
	ModulusID string
}

type FIDO2Req struct {
	AuthenticationOptions any
	ClientData            string
	AuthenticatorData     string
	Signature             string
	CredentialID          string
}

type AuthReq struct {
	Auth2FAReq `json:",omitempty"`

	Username        string
	ClientEphemeral string
	ClientProof     string
	SRPSession      string
}

type Auth struct {
	UserID string

	UID          string
	AccessToken  string
	RefreshToken string
	ServerProof  string

	Scope        string
	TwoFA        TwoFAInfo `json:"2FA"`
	PasswordMode PasswordMode
}

type RegisteredKey struct {
	AttestationFormat string
	CredentialID      []int
	Name              string
}

type FIDO2Info struct {
	AuthenticationOptions any
	RegisteredKeys        []RegisteredKey
}

type TwoFAInfo struct {
	Enabled TwoFAStatus
	FIDO2   FIDO2Info
}

type TwoFAStatus int

const (
	HasTOTP TwoFAStatus = 1 << iota
	HasFIDO2
)

type PasswordMode int

const (
	OnePasswordMode PasswordMode = iota + 1
	TwoPasswordMode
)

type Auth2FAReq struct {
	TwoFactorCode string   `json:",omitempty"`
	FIDO2         FIDO2Req `json:",omitempty"`
}

type AuthRefreshReq struct {
	UID          string
	RefreshToken string
	ResponseType string
	GrantType    string
	RedirectURI  string
	State        string
}

type AuthSession struct {
	UID        string
	CreateTime int64

	ClientID  string
	MemberID  string
	Revocable Bool

	LocalizedClientName string
}
