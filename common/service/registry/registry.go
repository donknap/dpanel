package registry

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/donknap/dpanel/common/service/docker"
	"github.com/donknap/dpanel/common/service/storage"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func New(opts ...Option) *Registry {
	c := &Registry{}
	for _, opt := range opts {
		opt(c)
	}
	c.Repository = &repository{
		registry: c,
	}
	return c
}

type Registry struct {
	authString string
	url        url.URL
	cacheTime  time.Duration
	Repository *repository
}

const (
	ScopeRepositoryPull = "repository:%s:pull"
	ChallengeHeader     = "WWW-Authenticate"
)

func (self Registry) accessToken(scope string) (string, error) {
	request, err := http.NewRequest("GET", self.url.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", docker.BuilderAuthor)

	client := &http.Client{}
	var response *http.Response
	if response, err = client.Do(request); err != nil {
		return "", err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	challenge := strings.ToLower(response.Header.Get(ChallengeHeader))
	slog.Debug("got response to challenge request", "status code", response.Status, "challenge", challenge)

	if strings.HasPrefix(challenge, "basic") {
		if self.authString == "" {
			return "", errors.New("no credentials available")
		}
		return fmt.Sprintf("Basic %s", self.authString), nil
	}

	if strings.HasPrefix(challenge, "bearer") {
		bearerUrl, err := self.getBearerUrl(challenge, scope)
		if err != nil {
			return "", err
		}

		var r *http.Request
		if r, err = http.NewRequest("GET", bearerUrl.String(), nil); err != nil {
			return "", err
		}
		if self.authString != "" {
			slog.Debug("Credentials found.")
			// CREDENTIAL: Uncomment to log registry credentials
			r.Header.Add("Authorization", fmt.Sprintf("Basic %s", self.authString))
		} else {
			slog.Debug("No credentials found.")
		}
		var authResponse *http.Response
		if authResponse, err = client.Do(r); err != nil {
			return "", err
		}
		body, _ := io.ReadAll(authResponse.Body)
		token := TokenResponse{}
		if err = json.Unmarshal(body, &token); err == nil {
			slog.Debug("registry", "access-token", token.AccessToken, "token", token.Token)
			if token.Token != "" {
				return fmt.Sprintf("Bearer %s", token.Token), nil
			}
			return fmt.Sprintf("Bearer %s", token.AccessToken), nil
		}
		return "", err
	}

	if challenge == "" {
		slog.Debug("registry access-token unsupported challenge type from registry")
	}
	return "", nil
}

func (self Registry) getBearerUrl(challenge string, scope string) (*url.URL, error) {
	loweredChallenge := strings.ToLower(challenge)
	raw := strings.TrimPrefix(loweredChallenge, "bearer")

	pairs := strings.Split(raw, ",")
	values := make(map[string]string, len(pairs))

	for _, pair := range pairs {
		trimmed := strings.Trim(pair, " ")
		if key, val, ok := strings.Cut(trimmed, "="); ok {
			values[key] = strings.Trim(val, `"`)
		}
	}
	if values["realm"] == "" || values["service"] == "" {
		return nil, errors.New("challenge header did not include all values needed to construct an auth url")
	}
	authURL, _ := url.Parse(values["realm"])
	q := authURL.Query()
	q.Add("service", values["service"])

	//scope := fmt.Sprintf("repository:%s:pull", "dpanel/dpanel")
	//scope := fmt.Sprintf("registry:catalog:pull")
	q.Add("scope", scope)
	authURL.RawQuery = q.Encode()
	slog.Debug("registry auth url", "url", authURL.String())
	return authURL, nil
}

func (self Registry) request(req *http.Request, scope string) (*http.Response, error) {
	cacheKey := fmt.Sprintf("registry:%s:%s", docker.Sdk.Name, req.URL.String())
	slog.Debug("registry request", "cacheKey", cacheKey, "scope", scope)

	if item, ok := storage.Cache.Get(cacheKey); self.cacheTime > 0 && ok {
		if c, ok := item.(cacheItem); ok {
			return &http.Response{
				Header: c.header,
				Body:   io.NopCloser(bytes.NewBuffer(c.body)),
			}, nil
		}
	}

	if token, err := self.accessToken(scope); err == nil {
		req.Header.Set("Authorization", token)
	} else {
		return nil, err
	}
	req.Header.Set("User-Agent", docker.BuilderAuthor)
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		wwwAuthHeader := res.Header.Get("www-authenticate")
		if wwwAuthHeader == "" {
			wwwAuthHeader = "not present"
		}
		return nil, fmt.Errorf("registry responded to head request with %q, auth: %q", res.Status, wwwAuthHeader)
	}
	slog.Debug("registry cache result", "cacheKey", req.URL.String())

	buffer := new(bytes.Buffer)
	_, _ = io.Copy(buffer, res.Body)

	storage.Cache.Set(cacheKey, cacheItem{
		header: res.Header,
		body:   buffer.Bytes(),
	}, self.cacheTime)

	res.Body = io.NopCloser(buffer)
	return res, nil
}
