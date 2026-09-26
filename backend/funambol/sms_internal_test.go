package funambol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	smsPhone   = "612345678"
	smsCode    = "123456"
	smsSession = "sid-1"
	smsData    = "data-1"
	smsNewID   = "sid-2"
	smsNewData = "data-2"
	smsPKCE    = "pkce-session"
	t3Consumer = "O2CLOUD_WEB" // client_name the O2 web client logs in as
)

// newSMSServer fakes the O2 server, the authorize endpoint and the T3
// credential API on one server.  callback is the redirectUri verify returns.
func newSMSServer(t *testing.T, callback func(srvURL string) string) configmap.Simple {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	old := t3API
	t3API = srv.URL + "/t3"
	t.Cleanup(func() { t3API = old })

	// The server keeps the PKCE verifier in the session this cookie names.
	mux.HandleFunc("/sapi/oauth/pkce/authorize", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "web", r.URL.Query().Get("platform"))
		http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: smsPKCE, Path: "/sapi"})
		http.Redirect(w, r, srv.URL+"/authorize", http.StatusFound)
	})
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/hop", http.StatusFound)
	})
	mux.HandleFunc("/hop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+t3Host+"/acceso/#/accessUserPassO2?client_name="+t3Consumer+
			"&sessionID="+smsSession+"&sessionData="+smsData, http.StatusFound)
	})

	mux.HandleFunc("/t3/manageCredentialMobileO2", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, t3Consumer+"/loginAppO2", r.Header.Get("COCO.idOrigen"))

		var req map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, map[string]string{
			"mobile": smsPhone, "sessionID": smsSession, "sessionData": smsData, "consumerId": t3Consumer,
		}, req)
		_, _ = w.Write([]byte(`{"newSessionData":"` + smsNewData + `","newSessionID":"` + smsNewID + `"}`))
	})
	mux.HandleFunc("/t3/verifyCredentialMobileO2", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		if req["otp"] != smsCode {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(t, map[string]string{
			"otp": smsCode, "sessionID": smsNewID, "sessionData": smsNewData, "Mobile": smsPhone,
		}, req)
		_, _ = w.Write([]byte(`{"redirectUri":"` + callback(srv.URL) + `"}`))
	})

	// The callback only completes the session that started the PKCE flow.
	mux.HandleFunc("/sapi/login/oauth", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("JSESSIONID")
		if err != nil || c.Value != smsPKCE {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "validationKey", Value: testKey, Path: "/"})
		http.Redirect(w, r, "/", http.StatusFound)
	})

	return configmap.Simple{"user": "+34 612 34 56 78", "endpoint": srv.URL}
}

// Without a password, rclone logs in with an SMS code via Telefónica's T3.
func TestConfigSMS(t *testing.T) {
	m := newSMSServer(t, func(srvURL string) string {
		return srvURL + "/sapi/login/oauth?code=c&state=s"
	})

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "sms", out.State)

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: smsCode})
	require.NoError(t, err)
	require.Nil(t, out)

	f, err := newSessionFs(context.Background(), "test", m)
	require.NoError(t, err)
	cookies, _ := m.Get("cookies")
	f.restoreCookies(cookies)
	assert.Equal(t, testKey, f.validationKeyFromJar())
}

// A redirectUri off the endpoint is refused rather than followed.
func TestConfigSMSForeignCallback(t *testing.T) {
	m := newSMSServer(t, func(string) string {
		return "https://evil.example.com/sapi/login/oauth?code=c&state=s"
	})

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: smsCode})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Contains(t, out.Error, "unexpected")
}

// A mistyped code can be entered again without a new SMS.
func TestConfigSMSWrongCode(t *testing.T) {
	m := newSMSServer(t, func(srvURL string) string {
		return srvURL + "/sapi/login/oauth?code=c&state=s"
	})

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.NoError(t, err)

	out, err := Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: "000000"})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "sms", out.State)

	out, err = Config(context.Background(), "test", m, fs.ConfigIn{State: "sms", Result: smsCode})
	require.NoError(t, err)
	assert.Nil(t, out)
}

// SMS login needs a mobile number; anything else fails before any request.
func TestConfigSMSNeedsMobile(t *testing.T) {
	m := newSMSServer(t, func(string) string { return "" })
	m["user"] = "someone@example.com"

	_, err := Config(context.Background(), "test", m, fs.ConfigIn{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mobile number")
}
