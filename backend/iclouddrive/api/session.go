package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/rest"
)

const iCloudUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3.1 Safari/605.1.15"

// Session represents an iCloud session
type Session struct {
	SessionToken   string         `json:"session_token"`
	Scnt           string         `json:"scnt"`
	SessionID      string         `json:"session_id"`
	AccountCountry string         `json:"account_country"`
	TrustToken     string         `json:"trust_token"`
	ClientID       string         `json:"client_id"`
	AuthAttributes string         `json:"auth_attributes"`
	FrameID        string         `json:"frame_id"`
	Cookies        []*http.Cookie `json:"cookies"`
	AccountInfo    AccountInfo    `json:"account_info"`

	srv         *rest.Client `json:"-"`
	needs2FA    bool         `json:"-"` // set when SRP signin returns 409
}

// srpInitResponse is the server response from /auth/signin/init
type srpInitResponse struct {
	Iteration int    `json:"iteration"`
	Salt      string `json:"salt"`
	Protocol  string `json:"protocol"`
	B         string `json:"b"`
	C         string `json:"c"`
}

// String returns the session as a string
// func (s *Session) String() string {
// 	jsession, _ := json.Marshal(s)
// 	return string(jsession)
// }

// Request makes a request
func (s *Session) Request(ctx context.Context, opts rest.Opts, request any, response any) (*http.Response, error) {
	resp, err := s.srv.CallJSON(ctx, &opts, &request, &response)

	if err != nil {
		return resp, err
	}

	if val := resp.Header.Get("X-Apple-ID-Account-Country"); val != "" {
		s.AccountCountry = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}
	if val := resp.Header.Get("X-Apple-Session-Token"); val != "" {
		s.SessionToken = val
	}
	if val := resp.Header.Get("X-Apple-TwoSV-Trust-Token"); val != "" {
		s.TrustToken = val
	}
	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-Auth-Attributes"); val != "" {
		s.AuthAttributes = val
	}

	return resp, nil
}

// Requires2FA returns true if the session requires 2FA
func (s *Session) Requires2FA() bool {
	if s.needs2FA {
		return true
	}
	return s.AccountInfo.DsInfo != nil && s.AccountInfo.DsInfo.HsaVersion == 2 && s.AccountInfo.HsaChallengeRequired
}

// SignIn performs SRP-based authentication against Apple's idmsa endpoint.
func (s *Session) SignIn(ctx context.Context, appleID, password string) error {
	// Step 1: Initialize the auth session
	if err := s.authStart(ctx); err != nil {
		return fmt.Errorf("authStart: %w", err)
	}

	// Step 2: Federate (submit account name)
	if err := s.authFederate(ctx, appleID); err != nil {
		return fmt.Errorf("authFederate: %w", err)
	}

	// Step 3: SRP init — send client public value A, get salt + B
	client := newSRPClient()
	aBase64 := base64.StdEncoding.EncodeToString(client.getABytes())

	initResp, err := s.authSRPInit(ctx, aBase64, appleID)
	if err != nil {
		return fmt.Errorf("authSRPInit: %w", err)
	}

	// Decode server values
	serverB, err := base64.StdEncoding.DecodeString(initResp.B)
	if err != nil {
		return fmt.Errorf("decode B: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("decode salt: %w", err)
	}

	// Step 4: Derive password key and process the SRP challenge
	derivedKey, err := derivePassword(password, salt, initResp.Iteration, initResp.Protocol)
	if err != nil {
		return fmt.Errorf("derivePassword: %w", err)
	}
	client.processChallenge([]byte(appleID), derivedKey, salt, serverB)

	// Step 5: Complete — send M1, M2 proofs
	m1Base64 := base64.StdEncoding.EncodeToString(client.M1)
	m2Base64 := base64.StdEncoding.EncodeToString(client.M2)

	if err := s.authSRPComplete(ctx, appleID, m1Base64, m2Base64, initResp.C); err != nil {
		return fmt.Errorf("authSRPComplete: %w", err)
	}

	return nil
}

// authStart initializes the SRP auth session by hitting the authorize/signin endpoint.
func (s *Session) authStart(ctx context.Context) error {
	if s.FrameID == "" {
		s.FrameID = strings.ToLower(uuid.New().String())
	}
	frameTag := "auth-" + s.FrameID

	params := url.Values{}
	params.Set("frame_id", frameTag)
	params.Set("language", "en_US")
	params.Set("skVersion", "7")
	params.Set("iframeId", frameTag)
	params.Set("client_id", s.ClientID)
	params.Set("redirect_uri", "https://www.icloud.com")
	params.Set("response_type", "code")
	params.Set("response_mode", "web_message")
	params.Set("state", frameTag)
	params.Set("authVersion", "latest")

	opts := rest.Opts{
		Method:     "GET",
		Path:       "/authorize/signin",
		Parameters: params,
		ExtraHeaders: map[string]string{
			"Accept":     "*/*",
			"User-Agent": iCloudUserAgent,
		},
		RootURL:    authEndpoint,
		NoResponse: true,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authStart: unexpected status %s", resp.Status)
	}

	if val := resp.Header.Get("X-Apple-Auth-Attributes"); val != "" {
		s.AuthAttributes = val
	}
	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}

	return nil
}

// authFederate submits the account name to Apple's federate endpoint.
func (s *Session) authFederate(ctx context.Context, accountName string) error {
	values := map[string]any{
		"accountName": accountName,
		"rememberMe":  true,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/federate",
		Parameters:   url.Values{"isRememberMeEnabled": {"true"}},
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	if val := resp.Header.Get("X-Apple-Auth-Attributes"); val != "" {
		s.AuthAttributes = val
	}
	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authFederate: unexpected status %s", resp.Status)
	}
	return nil
}

// authSRPInit sends the client's public value A to the server and retrieves
// the salt, server public value B, iteration count, protocol, and challenge.
func (s *Session) authSRPInit(ctx context.Context, aBase64, accountName string) (*srpInitResponse, error) {
	values := map[string]any{
		"a":           aBase64,
		"accountName": accountName,
		"protocols":   []string{"s2k", "s2k_fo"},
	}
	body, err := IntoReader(values)
	if err != nil {
		return nil, err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/signin/init",
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		Body:         body,
	}

	var initResp srpInitResponse
	resp, err := s.srv.CallJSON(ctx, &opts, nil, &initResp)
	if err != nil {
		return nil, err
	}

	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}

	return &initResp, nil
}

// authSRPComplete sends the SRP proofs M1 and M2 to complete authentication.
// Returns nil on success (200 or 409/2FA needed).
func (s *Session) authSRPComplete(ctx context.Context, accountName, m1Base64, m2Base64, c string) error {
	trustTokens := []string{}
	if s.TrustToken != "" {
		trustTokens = []string{s.TrustToken}
	}

	values := map[string]any{
		"accountName":  accountName,
		"m1":           m1Base64,
		"m2":           m2Base64,
		"c":            c,
		"rememberMe":   true,
		"trustTokens":  trustTokens,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/signin/complete",
		Parameters:   url.Values{"isRememberMeEnabled": {"true"}},
		ExtraHeaders: s.getSRPAuthHeaders(),
		RootURL:      authEndpoint,
		IgnoreStatus: true,
		NoResponse:   true,
		Body:         body,
	}

	resp, err := s.srv.Call(ctx, &opts)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	// Extract updated headers
	if val := resp.Header.Get("X-Apple-Auth-Attributes"); val != "" {
		s.AuthAttributes = val
	}
	if val := resp.Header.Get("X-Apple-Session-Token"); val != "" {
		s.SessionToken = val
	}
	if val := resp.Header.Get("scnt"); val != "" {
		s.Scnt = val
	}
	if val := resp.Header.Get("X-Apple-ID-Session-Id"); val != "" {
		s.SessionID = val
	}
	if val := resp.Header.Get("X-Apple-ID-Account-Country"); val != "" {
		s.AccountCountry = val
	}
	if val := resp.Header.Get("X-Apple-TwoSV-Trust-Token"); val != "" {
		s.TrustToken = val
	}

	switch resp.StatusCode {
	case http.StatusOK:
		fs.Debugf("icloud", "SRP sign in successful")
		return nil
	case http.StatusConflict:
		// 409 = 2FA required, this is expected
		fs.Debugf("icloud", "SRP sign in requires 2FA")
		s.needs2FA = true
		return nil
	case http.StatusForbidden:
		return fmt.Errorf("sign in failed: incorrect username or password")
	default:
		return fmt.Errorf("sign in failed: %s", resp.Status)
	}
}

// getSRPAuthHeaders returns headers needed for SRP auth requests to idmsa.apple.com.
func (s *Session) getSRPAuthHeaders() map[string]string {
	frameTag := "auth-" + s.FrameID
	headers := map[string]string{
		"Accept":                           "application/json",
		"Content-Type":                     "application/json",
		"User-Agent":                       iCloudUserAgent,
		"Origin":                           "https://idmsa.apple.com",
		"Referer":                          "https://idmsa.apple.com/",
		"X-Apple-Widget-Key":               s.ClientID,
		"X-Apple-OAuth-Client-Id":          s.ClientID,
		"X-Apple-OAuth-Client-Type":        "firstPartyAuth",
		"X-Apple-OAuth-Redirect-URI":       "https://www.icloud.com",
		"X-Apple-OAuth-Require-Grant-Code": "true",
		"X-Apple-OAuth-Response-Mode":      "web_message",
		"X-Apple-OAuth-Response-Type":      "code",
		"X-Apple-OAuth-State":              frameTag,
		"X-Apple-Frame-Id":                 frameTag,
		"X-Requested-With":                 "XMLHttpRequest",
		"X-Apple-Mandate-Security-Upgrade": "0",
		"X-Apple-I-Require-UE":             "true",
		"X-Apple-I-FD-Client-Info":         `{"U":"` + iCloudUserAgent + `","L":"en-US","Z":"GMT-05:00","V":"1.1","F":""}`,
	}
	if s.AuthAttributes != "" {
		headers["X-Apple-Auth-Attributes"] = s.AuthAttributes
	}
	if s.Scnt != "" {
		headers["scnt"] = s.Scnt
	}
	if s.SessionID != "" {
		headers["X-Apple-ID-Session-Id"] = s.SessionID
	}
	return headers
}

// AuthWithToken authenticates the session
func (s *Session) AuthWithToken(ctx context.Context) error {
	values := map[string]any{
		"accountCountryCode": s.AccountCountry,
		"dsWebAuthToken":     s.SessionToken,
		"extended_login":     true,
		"trustToken":         s.TrustToken,
	}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/accountLogin",
		ExtraHeaders: GetCommonHeaders(map[string]string{}),
		RootURL:      setupEndpoint,
		Body:         body,
	}

	resp, err := s.Request(ctx, opts, nil, &s.AccountInfo)
	if err == nil {
		s.Cookies = resp.Cookies()
	}

	return err
}

// Validate2FACode validates the 2FA code
func (s *Session) Validate2FACode(ctx context.Context, code string) error {
	values := map[string]any{"securityCode": map[string]string{"code": code}}
	body, err := IntoReader(values)
	if err != nil {
		return err
	}

	opts := rest.Opts{
		Method:       "POST",
		Path:         "/verify/trusteddevice/securitycode",
		ExtraHeaders: s.GetAuthHeaders(map[string]string{}),
		RootURL:      authEndpoint,
		Body:         body,
		NoResponse:   true,
	}

	_, err = s.Request(ctx, opts, nil, nil)
	if err == nil {
		if err := s.TrustSession(ctx); err != nil {
			return err
		}

		return nil
	}

	return fmt.Errorf("validate2FACode failed: %w", err)
}

// TrustSession trusts the session
func (s *Session) TrustSession(ctx context.Context) error {
	opts := rest.Opts{
		Method:        "GET",
		Path:          "/2sv/trust",
		ExtraHeaders:  s.GetAuthHeaders(map[string]string{}),
		RootURL:       authEndpoint,
		NoResponse:    true,
		ContentLength: common.Int64(0),
	}

	_, err := s.Request(ctx, opts, nil, nil)
	if err != nil {
		return fmt.Errorf("trustSession failed: %w", err)
	}

	return s.AuthWithToken(ctx)
}

// ValidateSession validates the session
func (s *Session) ValidateSession(ctx context.Context) error {
	opts := rest.Opts{
		Method:        "POST",
		Path:          "/validate",
		ExtraHeaders:  s.GetHeaders(map[string]string{}),
		RootURL:       setupEndpoint,
		ContentLength: common.Int64(0),
	}
	_, err := s.Request(ctx, opts, nil, &s.AccountInfo)
	if err != nil {
		return fmt.Errorf("validateSession failed: %w", err)
	}

	return nil
}

// GetAuthHeaders returns the authentication headers for the session.
// Used for 2FA validation and trust requests to idmsa.apple.com.
func (s *Session) GetAuthHeaders(overwrite map[string]string) map[string]string {
	headers := s.getSRPAuthHeaders()
	maps.Copy(headers, overwrite)
	return headers
}

// GetHeaders Gets the authentication headers required for a request
func (s *Session) GetHeaders(overwrite map[string]string) map[string]string {
	headers := GetCommonHeaders(map[string]string{})
	headers["Cookie"] = s.GetCookieString()
	maps.Copy(headers, overwrite)
	return headers
}

// GetCookieString returns the cookie header string for the session.
func (s *Session) GetCookieString() string {
	cookieHeader := ""
	// we only care about name and value.
	for _, cookie := range s.Cookies {
		cookieHeader = cookieHeader + cookie.Name + "=" + cookie.Value + ";"
	}
	return cookieHeader
}

// GetCommonHeaders generates common HTTP headers with optional overwrite.
func GetCommonHeaders(overwrite map[string]string) map[string]string {
	headers := map[string]string{
		"Content-Type": "application/json",
		"Origin":       baseEndpoint,
		"Referer":      fmt.Sprintf("%s/", baseEndpoint),
		"User-Agent":   iCloudUserAgent,
	}
	maps.Copy(headers, overwrite)
	return headers
}

// MergeCookies merges two slices of http.Cookies, ensuring no duplicates are added.
func MergeCookies(left []*http.Cookie, right []*http.Cookie) ([]*http.Cookie, error) {
	var hashes []string
	for _, cookie := range right {
		hashes = append(hashes, cookie.Raw)
	}
	for _, cookie := range left {
		if !slices.Contains(hashes, cookie.Raw) {
			right = append(right, cookie)
		}
	}
	return right, nil
}

// GetCookiesForDomain filters the provided cookies based on the domain of the given URL.
func GetCookiesForDomain(url *url.URL, cookies []*http.Cookie) ([]*http.Cookie, error) {
	var domainCookies []*http.Cookie
	for _, cookie := range cookies {
		if strings.HasSuffix(url.Host, cookie.Domain) {
			domainCookies = append(domainCookies, cookie)
		}
	}
	return domainCookies, nil
}

// NewSession creates a new Session instance with default values.
func NewSession() *Session {
	session := &Session{
		FrameID: strings.ToLower(uuid.New().String()),
	}
	httpClient := fshttp.NewClient(context.Background())
	if tr, ok := httpClient.Transport.(*fshttp.Transport); ok {
		tr.SetRequestFilter(func(req *http.Request) {
			req.Header.Set("User-Agent", iCloudUserAgent)
		})
	}
	session.srv = rest.NewClient(httpClient).SetRoot(baseEndpoint)
	return session
}

// AccountInfo represents an account info
type AccountInfo struct {
	DsInfo                       *ValidateDataDsInfo    `json:"dsInfo"`
	HasMinimumDeviceForPhotosWeb bool                   `json:"hasMinimumDeviceForPhotosWeb"`
	ICDPEnabled                  bool                   `json:"iCDPEnabled"`
	Webservices                  map[string]*webService `json:"webservices"`
	PcsEnabled                   bool                   `json:"pcsEnabled"`
	TermsUpdateNeeded            bool                   `json:"termsUpdateNeeded"`
	ConfigBag                    struct {
		Urls struct {
			AccountCreateUI     string `json:"accountCreateUI"`
			AccountLoginUI      string `json:"accountLoginUI"`
			AccountLogin        string `json:"accountLogin"`
			AccountRepairUI     string `json:"accountRepairUI"`
			DownloadICloudTerms string `json:"downloadICloudTerms"`
			RepairDone          string `json:"repairDone"`
			AccountAuthorizeUI  string `json:"accountAuthorizeUI"`
			VettingURLForEmail  string `json:"vettingUrlForEmail"`
			AccountCreate       string `json:"accountCreate"`
			GetICloudTerms      string `json:"getICloudTerms"`
			VettingURLForPhone  string `json:"vettingUrlForPhone"`
		} `json:"urls"`
		AccountCreateEnabled bool `json:"accountCreateEnabled"`
	} `json:"configBag"`
	HsaTrustedBrowser            bool     `json:"hsaTrustedBrowser"`
	AppsOrder                    []string `json:"appsOrder"`
	Version                      int      `json:"version"`
	IsExtendedLogin              bool     `json:"isExtendedLogin"`
	PcsServiceIdentitiesIncluded bool     `json:"pcsServiceIdentitiesIncluded"`
	IsRepairNeeded               bool     `json:"isRepairNeeded"`
	HsaChallengeRequired         bool     `json:"hsaChallengeRequired"`
	RequestInfo                  struct {
		Country  string `json:"country"`
		TimeZone string `json:"timeZone"`
		Region   string `json:"region"`
	} `json:"requestInfo"`
	PcsDeleted bool `json:"pcsDeleted"`
	ICloudInfo struct {
		SafariBookmarksHasMigratedToCloudKit bool `json:"SafariBookmarksHasMigratedToCloudKit"`
	} `json:"iCloudInfo"`
	Apps map[string]*ValidateDataApp `json:"apps"`
}

// ValidateDataDsInfo represents an validation info
type ValidateDataDsInfo struct {
	HsaVersion                         int      `json:"hsaVersion"`
	LastName                           string   `json:"lastName"`
	ICDPEnabled                        bool     `json:"iCDPEnabled"`
	TantorMigrated                     bool     `json:"tantorMigrated"`
	Dsid                               string   `json:"dsid"`
	HsaEnabled                         bool     `json:"hsaEnabled"`
	IsHideMyEmailSubscriptionActive    bool     `json:"isHideMyEmailSubscriptionActive"`
	IroncadeMigrated                   bool     `json:"ironcadeMigrated"`
	Locale                             string   `json:"locale"`
	BrZoneConsolidated                 bool     `json:"brZoneConsolidated"`
	ICDRSCapableDeviceList             string   `json:"ICDRSCapableDeviceList"`
	IsManagedAppleID                   bool     `json:"isManagedAppleID"`
	IsCustomDomainsFeatureAvailable    bool     `json:"isCustomDomainsFeatureAvailable"`
	IsHideMyEmailFeatureAvailable      bool     `json:"isHideMyEmailFeatureAvailable"`
	ContinueOnDeviceEligibleDeviceInfo []string `json:"ContinueOnDeviceEligibleDeviceInfo"`
	Gilligvited                        bool     `json:"gilligvited"`
	AppleIDAliases                     []any    `json:"appleIdAliases"`
	UbiquityEOLEnabled                 bool     `json:"ubiquityEOLEnabled"`
	IsPaidDeveloper                    bool     `json:"isPaidDeveloper"`
	CountryCode                        string   `json:"countryCode"`
	NotificationID                     string   `json:"notificationId"`
	PrimaryEmailVerified               bool     `json:"primaryEmailVerified"`
	ADsID                              string   `json:"aDsID"`
	Locked                             bool     `json:"locked"`
	ICDRSCapableDeviceCount            int      `json:"ICDRSCapableDeviceCount"`
	HasICloudQualifyingDevice          bool     `json:"hasICloudQualifyingDevice"`
	PrimaryEmail                       string   `json:"primaryEmail"`
	AppleIDEntries                     []struct {
		IsPrimary bool   `json:"isPrimary"`
		Type      string `json:"type"`
		Value     string `json:"value"`
	} `json:"appleIdEntries"`
	GilliganEnabled    bool   `json:"gilligan-enabled"`
	IsWebAccessAllowed bool   `json:"isWebAccessAllowed"`
	FullName           string `json:"fullName"`
	MailFlags          struct {
		IsThreadingAvailable           bool `json:"isThreadingAvailable"`
		IsSearchV2Provisioned          bool `json:"isSearchV2Provisioned"`
		SCKMail                        bool `json:"sCKMail"`
		IsMppSupportedInCurrentCountry bool `json:"isMppSupportedInCurrentCountry"`
	} `json:"mailFlags"`
	LanguageCode         string `json:"languageCode"`
	AppleID              string `json:"appleId"`
	HasUnreleasedOS      bool   `json:"hasUnreleasedOS"`
	AnalyticsOptInStatus bool   `json:"analyticsOptInStatus"`
	FirstName            string `json:"firstName"`
	ICloudAppleIDAlias   string `json:"iCloudAppleIdAlias"`
	NotesMigrated        bool   `json:"notesMigrated"`
	BeneficiaryInfo      struct {
		IsBeneficiary bool `json:"isBeneficiary"`
	} `json:"beneficiaryInfo"`
	HasPaymentInfo bool   `json:"hasPaymentInfo"`
	PcsDelet       bool   `json:"pcsDelet"`
	AppleIDAlias   string `json:"appleIdAlias"`
	BrMigrated     bool   `json:"brMigrated"`
	StatusCode     int    `json:"statusCode"`
	FamilyEligible bool   `json:"familyEligible"`
}

// ValidateDataApp represents an app
type ValidateDataApp struct {
	CanLaunchWithOneFactor bool `json:"canLaunchWithOneFactor"`
	IsQualifiedForBeta     bool `json:"isQualifiedForBeta"`
}

// WebService represents a web service
type webService struct {
	PcsRequired bool   `json:"pcsRequired"`
	URL         string `json:"url"`
	UploadURL   string `json:"uploadUrl"`
	Status      string `json:"status"`
}
