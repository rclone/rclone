//go:build !plan9 && !solaris && !js
// +build !plan9,!solaris,!js

package azureblob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func handler(t *testing.T, actual *map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		require.NoError(t, err)
		parameters := r.URL.Query()
		(*actual)["path"] = r.URL.Path
		(*actual)["Metadata"] = r.Header.Get("Metadata")
		(*actual)["method"] = r.Method
		for paramName := range parameters {
			(*actual)[paramName] = parameters.Get(paramName)
		}
		// Make response.
		response := adal.Token{}
		responseBytes, err := json.Marshal(response)
		require.NoError(t, err)
		_, err = w.Write(responseBytes)
		require.NoError(t, err)
	}
}

func TestManagedIdentity(t *testing.T) {
	// test user-assigned identity specifiers to use
	testMSIClientID := "d859b29f-5c9c-42f8-a327-ec1bc6408d79"
	testMSIObjectID := "9ffeb650-3ca0-4278-962b-5a38d520591a"
	testMSIResourceID := "/subscriptions/fe714c49-b8a4-4d49-9388-96a20daa318f/resourceGroups/somerg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/someidentity"
	tests := []struct {
		identity              *userMSI
		identityParameterName string
		expectedAbsent        []string
	}{
		{&userMSI{msiClientID, testMSIClientID}, "client_id", []string{"object_id", "mi_res_id"}},
		{&userMSI{msiObjectID, testMSIObjectID}, "object_id", []string{"client_id", "mi_res_id"}},
		{&userMSI{msiResourceID, testMSIResourceID}, "mi_res_id", []string{"object_id", "client_id"}},
		{nil, "(default)", []string{"object_id", "client_id", "mi_res_id"}},
	}
	alwaysExpected := map[string]string{
		"path":        "/metadata/identity/oauth2/token",
		"resource":    "https://storage.azure.com",
		"Metadata":    "true",
		"api-version": "2018-02-01",
		"method":      "GET",
	}
	for _, test := range tests {
		actual := make(map[string]string, 10)
		testServer := httptest.NewServer(handler(t, &actual))
		defer testServer.Close()
		testServerPort, err := strconv.Atoi(strings.Split(testServer.URL, ":")[2])
		require.NoError(t, err)
		ctx := context.WithValue(context.TODO(), testPortKey("testPort"), testServerPort)
		_, err = GetMSIToken(ctx, test.identity)
		require.NoError(t, err)

		// Validate expected query parameters present
		expected := make(map[string]string)
		for k, v := range alwaysExpected {
			expected[k] = v
		}
		if test.identity != nil {
			expected[test.identityParameterName] = test.identity.Value
		}

		for key := range expected {
			value, exists := actual[key]
			if assert.Truef(t, exists, "test of %s: query parameter %s was not passed",
				test.identityParameterName, key) {
				assert.Equalf(t, expected[key], value,
					"test of %s: parameter %s has incorrect value", test.identityParameterName, key)
			}
		}

		// Validate unexpected query parameters absent
		for _, key := range test.expectedAbsent {
			_, exists := actual[key]
			assert.Falsef(t, exists, "query parameter %s was unexpectedly passed")
		}
	}
}

func errorHandler(resultCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Test error generated", resultCode)
	}
}

func TestIMDSErrors(t *testing.T) {
	errorCodes := []int{404, 429, 500}
	for _, code := range errorCodes {
		testServer := httptest.NewServer(errorHandler(code))
		defer testServer.Close()
		testServerPort, err := strconv.Atoi(strings.Split(testServer.URL, ":")[2])
		require.NoError(t, err)
		ctx := context.WithValue(context.TODO(), testPortKey("testPort"), testServerPort)
		_, err = GetMSIToken(ctx, nil)
		require.Error(t, err)
		httpErr, ok := err.(httpError)
		require.Truef(t, ok, "HTTP error %d did not result in an httpError object", code)
		assert.Equalf(t, httpErr.Response.StatusCode, code, "desired error %d but didn't get it", code)
	}
}
