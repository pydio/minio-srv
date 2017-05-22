/*
 * Minio Cloud Storage, (C) 2016, 2017 Minio, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/anaskhan96/soup"
	jwtgo "github.com/dgrijalva/jwt-go"
	jwtreq "github.com/dgrijalva/jwt-go/request"
	"github.com/levigross/grequests"
)

const (
	jwtAlgorithm = "Bearer"

	// Default JWT token for web handlers is one day.
	defaultJWTExpiry = 24 * time.Hour

	// Inter-node JWT token expiry is 100 years approx.
	defaultInterNodeJWTExpiry = 100 * 365 * 24 * time.Hour
)

var (
	errInvalidAccessKeyID   = errors.New("The access key ID you provided does not exist in our records")
	errChangeCredNotAllowed = errors.New("Changing access key and secret key not allowed")
	errAuthentication       = errors.New("Authentication failed, check your access credentials")
	errNoAuthToken          = errors.New("JWT token missing")
)

func getURL(u *url.URL) string {
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
}

func getSAMLAssertion(username, password string, saml samlProvider) (string, error) {
	httpSess := grequests.NewSession(nil)

	u, err := url.Parse(saml.IDP)
	if err != nil {
		return "", err
	}
	v := url.Values{
		"providerId": {saml.ProviderID},
	}
	u.RawQuery = v.Encode()

	resp, err := httpSess.Get(u.String(), nil)
	if err != nil {
		return "", err
	}

	samlLogin := soup.HTMLParse(resp.String())
	resp.Close()

	payload := extractPayload(samlLogin)
	payload["username"] = username
	payload["password"] = password
	resp, err = httpSess.Post(getURL(resp.RawResponse.Request.URL),
		&grequests.RequestOptions{
			Data:         payload,
			UseCookieJar: true,
		},
	)
	if err != nil {
		return "", err
	}

	samlAssertion := soup.HTMLParse(resp.String())
	resp.Close()

	return extractSAMLAssertion(samlAssertion), nil
}

func authenticateJWTWithSAML(accessKey, secretKey string, expiry time.Duration, saml samlProvider) (string, error) {
	samlAssertion, err := getSAMLAssertion(accessKey, secretKey, saml)
	if err != nil {
		return "", err
	}

	samlResp, err := ParseSAMLResponse(samlAssertion)
	if err != nil {
		return "", err
	}

	// Keep TLS config.
	tlsConfig := &tls.Config{
		RootCAs:            globalRootCAs,
		InsecureSkipVerify: true,
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig:       tlsConfig,
		},
	}

	resp, rerr := client.PostForm(samlResp.Destination, url.Values{
		"SAMLResponse": {samlResp.origSAMLAssertion},
	})
	if rerr != nil {
		return "", rerr
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return "", errors.New(resp.Status)
	}

	expiryTime := UTCNow().Add(expiry)
	cred, err := getNewCredentialWithExpiry(expiryTime)
	if err != nil {
		return "", err
	}

	utcNow := UTCNow()
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS512, jwtgo.MapClaims{
		"exp": utcNow.Add(expiry).Unix(),
		"iat": utcNow.Unix(),
		"sub": cred.AccessKey,
	})

	serverConfig.SetCredential(cred)
	return token.SignedString([]byte(cred.SecretKey))
}

func authenticateJWT(accessKey, secretKey string, expiry time.Duration) (string, error) {
	passedCredential, err := createCredential(accessKey, secretKey)
	if err != nil {
		return "", err
	}

	serverCred := serverConfig.GetCredential()
	if serverCred.AccessKey != passedCredential.AccessKey {
		return "", errInvalidAccessKeyID
	}

	if !serverCred.Equal(passedCredential) {
		return "", errAuthentication
	}

	utcNow := UTCNow()
	token := jwtgo.NewWithClaims(jwtgo.SigningMethodHS512, jwtgo.MapClaims{
		"exp": utcNow.Add(expiry).Unix(),
		"iat": utcNow.Unix(),
		"sub": accessKey,
	})

	return token.SignedString([]byte(serverCred.SecretKey))
}

func authenticateNode(accessKey, secretKey string) (string, error) {
	return authenticateJWT(accessKey, secretKey, defaultInterNodeJWTExpiry)
}

func authenticateWeb(accessKey, secretKey string) (string, error) {
	saml := serverConfig.Auth.GetSAMLByID("1") // TODO: Needs to be configurable.
	if saml.Enable {
		return authenticateJWTWithSAML(accessKey, secretKey, defaultJWTExpiry, saml)
	}
	return authenticateJWT(accessKey, secretKey, defaultJWTExpiry)
}

func keyFuncCallback(jwtToken *jwtgo.Token) (interface{}, error) {
	if _, ok := jwtToken.Method.(*jwtgo.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", jwtToken.Header["alg"])
	}

	return []byte(serverConfig.GetCredential().SecretKey), nil
}

func isAuthTokenValid(tokenString string) bool {
	jwtToken, err := jwtgo.Parse(tokenString, keyFuncCallback)
	if err != nil {
		errorIf(err, "Unable to parse JWT token string")
		return false
	}

	return jwtToken.Valid
}

func isHTTPRequestValid(req *http.Request) bool {
	return webRequestAuthenticate(req) == nil
}

// Check if the request is authenticated.
// Returns nil if the request is authenticated. errNoAuthToken if token missing.
// Returns errAuthentication for all other errors.
func webRequestAuthenticate(req *http.Request) error {
	jwtToken, err := jwtreq.ParseFromRequest(req, jwtreq.AuthorizationHeaderExtractor, keyFuncCallback)
	if err != nil {
		if err == jwtreq.ErrNoTokenInRequest {
			return errNoAuthToken
		}
		return errAuthentication
	}

	if !jwtToken.Valid {
		return errAuthentication
	}
	return nil
}
