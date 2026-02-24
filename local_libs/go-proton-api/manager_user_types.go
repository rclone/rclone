package proton

type TokenType string

const (
	EmailTokenType TokenType = "email"
	SMSTokenType   TokenType = "sms"
)

type SendVerificationCodeReq struct {
	Username    string
	Type        TokenType
	Destination TokenDestination
}

type TokenDestination struct {
	Address string
	Phone   string
}

type UserType int

const (
	MailUserType UserType = iota + 1
	VPNUserType
)

type CreateUserReq struct {
	Type     UserType
	Username string
	Domain   string
	Auth     AuthVerifier
}
