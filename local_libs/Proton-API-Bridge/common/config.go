package common

import (
	"os"
	"runtime"
)

type Config struct {
	/* Constants */
	AppVersion string
	UserAgent  string

	/* Login */
	FirstLoginCredential *FirstLoginCredentialData
	ReusableCredential   *ReusableCredentialData
	UseReusableLogin     bool
	CredentialCacheFile  string // If CredentialCacheFile is empty, no credential will be logged

	/* Setting */
	DestructiveIntegrationTest     bool // CAUTION: the integration test requires a clean proton drive
	EmptyTrashAfterIntegrationTest bool // CAUTION: the integration test will clean up all the data in the trash
	ReplaceExistingDraft           bool // for the file upload replace or keep it as-is option
	EnableCaching                  bool // link node caching
	ConcurrentBlockUploadCount     int
	ConcurrentFileCryptoCount      int

	/* Drive */
	DataFolderName string
}

type FirstLoginCredentialData struct {
	Username        string
	Password        string
	MailboxPassword string
	TwoFA           string
}

type ReusableCredentialData struct {
	UID           string
	AccessToken   string
	RefreshToken  string
	SaltedKeyPass string // []byte <-> base64
}

func NewConfigWithDefaultValues() *Config {
	return &Config{
		AppVersion: "",
		UserAgent:  "",

		FirstLoginCredential: &FirstLoginCredentialData{
			Username:        "",
			Password:        "",
			MailboxPassword: "",
			TwoFA:           "",
		},
		ReusableCredential: &ReusableCredentialData{
			UID:           "",
			AccessToken:   "",
			RefreshToken:  "",
			SaltedKeyPass: "", // []byte <-> base64
		},
		UseReusableLogin:    false,
		CredentialCacheFile: "",

		DestructiveIntegrationTest:     false,
		EmptyTrashAfterIntegrationTest: false,
		ReplaceExistingDraft:           false,
		EnableCaching:                  true,
		ConcurrentBlockUploadCount:     20, // let's be a nice citizen and not stress out proton engineers :)
		ConcurrentFileCryptoCount:      runtime.GOMAXPROCS(0),

		DataFolderName: "data",
	}
}

func NewConfigForIntegrationTests() *Config {
	appVersion := os.Getenv("PROTON_API_BRIDGE_APP_VERSION")
	userAgent := os.Getenv("PROTON_API_BRIDGE_USER_AGENT")

	username := os.Getenv("PROTON_API_BRIDGE_TEST_USERNAME")
	password := os.Getenv("PROTON_API_BRIDGE_TEST_PASSWORD")
	twoFA := os.Getenv("PROTON_API_BRIDGE_TEST_TWOFA")

	useReusableLoginStr := os.Getenv("PROTON_API_BRIDGE_TEST_USE_REUSABLE_LOGIN")
	useReusableLogin := false
	if useReusableLoginStr == "1" {
		useReusableLogin = true
	}

	uid := os.Getenv("PROTON_API_BRIDGE_TEST_UID")
	accessToken := os.Getenv("PROTON_API_BRIDGE_TEST_ACCESS_TOKEN")
	refreshToken := os.Getenv("PROTON_API_BRIDGE_TEST_REFRESH_TOKEN")
	saltedKeyPass := os.Getenv("PROTON_API_BRIDGE_TEST_SALTEDKEYPASS")

	return &Config{
		AppVersion: appVersion,
		UserAgent:  userAgent,

		FirstLoginCredential: &FirstLoginCredentialData{
			Username:        username,
			Password:        password,
			MailboxPassword: "",
			TwoFA:           twoFA,
		},
		ReusableCredential: &ReusableCredentialData{
			UID:           uid,
			AccessToken:   accessToken,
			RefreshToken:  refreshToken,
			SaltedKeyPass: saltedKeyPass, // []byte <-> base64
		},
		UseReusableLogin:    useReusableLogin,
		CredentialCacheFile: ".credential",

		DestructiveIntegrationTest:     true,
		EmptyTrashAfterIntegrationTest: true,
		ReplaceExistingDraft:           false,
		EnableCaching:                  true,
		ConcurrentBlockUploadCount:     20,
		ConcurrentFileCryptoCount:      runtime.GOMAXPROCS(0),

		DataFolderName: "data",
	}
}
