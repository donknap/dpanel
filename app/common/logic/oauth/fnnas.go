package oauth

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	commonLogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/google/uuid"
	"github.com/we7coreteam/w7-rangine-go/v2/pkg/support/facade"
	"gorm.io/gorm"
)

const (
	fnnasClientID         = "dpanel"
	fnnasResponseTypeCode = "code"
	fnnasCodeTTL          = time.Minute
	fnnasStateTTL         = 5 * time.Minute
)

var fnnasExchangeLock sync.Mutex

type Fnnas struct {
}

type FnnasUser struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

type FnnasCode struct {
	Code        string
	Provider    string
	RedirectURI string
	State       string
	User        FnnasUser
	ExpiresAt   time.Time
	Used        bool
}

func (self Fnnas) Item() (Item, bool) {
	// FNNAS login is initiated by the FNNAS system, so it is not shown on the DPanel login page.
	return Item{}, false
}

func (self Fnnas) Authorize(request *http.Request) (string, error) {
	enabled := self.Enable()
	if !enabled {
		return "", errors.New("fnnas oauth is not enabled")
	}
	if err := self.ValidateAuthorizeRequest(request); err != nil {
		slog.Debug("fnnas oauth authorize validate failed", "error", err.Error())
		return "", err
	}
	return self.AuthorizeByGateway(request)
}

func (self Fnnas) Exchange(option ExchangeOption) (string, error) {
	fnnasExchangeLock.Lock()
	defer fnnasExchangeLock.Unlock()
	cacheKey := fmt.Sprintf(storage.CacheKeyOauthCode, option.Code)
	item, exists := storage.Cache.Get(cacheKey)
	if !exists {
		slog.Debug("fnnas oauth exchange failed", "reason", "code cache missing")
		return "", errors.New("oauth code is invalid")
	}
	codeInfo, ok := item.(*FnnasCode)
	if !ok || codeInfo.Provider != ProviderFnnas {
		slog.Debug("fnnas oauth exchange failed", "reason", "code cache type or provider invalid")
		return "", errors.New("oauth code is invalid")
	}
	if codeInfo.Used {
		slog.Debug("fnnas oauth exchange failed", "reason", "code used")
		return "", errors.New("oauth code has been used")
	}
	if time.Now().After(codeInfo.ExpiresAt) {
		storage.Cache.Delete(cacheKey)
		slog.Debug("fnnas oauth exchange failed", "reason", "code expired", "expiresAt", codeInfo.ExpiresAt)
		return "", errors.New("oauth code has expired")
	}
	if option.State == "" || codeInfo.State != option.State {
		slog.Debug("fnnas oauth exchange failed", "reason", "state mismatch")
		return "", errors.New("oauth state is invalid")
	}
	if option.RedirectURI != "" && codeInfo.RedirectURI != option.RedirectURI {
		slog.Debug("fnnas oauth exchange failed", "reason", "redirect uri mismatch", "cachedRedirectURI", codeInfo.RedirectURI, "requestRedirectURI", option.RedirectURI)
		return "", errors.New("oauth redirect uri is invalid")
	}

	founder, err := commonLogic.User{}.GetFounderUser()
	if err != nil {
		return "", err
	}
	codeInfo.Used = true
	storage.Cache.Set(cacheKey, codeInfo, time.Second)
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyOauthState, codeInfo.State))
	slog.Debug("fnnas oauth exchange success")
	return commonLogic.User{}.GetUserOauthToken(founder, false)
}

func (self Fnnas) Enable() bool {
	return os.Getenv("DP_RUN_IN_FNNAS") == "1"
}

func (self Fnnas) ValidateAuthorizeRequest(request *http.Request) error {
	responseType := request.URL.Query().Get("response_type")
	if responseType != "" && responseType != fnnasResponseTypeCode {
		return errors.New("oauth response type is invalid")
	}
	clientID := request.URL.Query().Get("client_id")
	if clientID != "" && clientID != fnnasClientID {
		return errors.New("oauth client id is invalid")
	}
	if request.URL.Query().Get("redirect_uri") != "" || request.URL.Query().Get("state") != "" {
		return errors.New("oauth request is invalid")
	}
	return nil
}

func (self Fnnas) AuthorizeByGateway(request *http.Request) (string, error) {
	isTcpRequest := self.IsTcpRequest(request)
	if isTcpRequest {
		return "", errors.New("fnnas authorize only supports unix socket requests")
	}
	fnnasUser, err := self.User(request)
	if err != nil {
		slog.Debug("fnnas oauth authorize gateway failed", "reason", "user header invalid", "error", err.Error())
		return "", err
	}
	if !fnnasUser.IsAdmin {
		slog.Debug("fnnas oauth authorize gateway failed", "reason", "user is not admin")
		return "", errors.New("fnnas user is not administrator")
	}
	if _, err = (commonLogic.User{}).GetFounderUser(); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if _, err = (commonLogic.User{}).CreateFounderUser(fnnasUser.Username, uuid.NewString()); err != nil {
				slog.Debug("fnnas oauth authorize gateway failed", "reason", "founder create failed", "error", err.Error())
				return "", err
			}
		} else {
			slog.Debug("fnnas oauth authorize gateway failed", "reason", "founder invalid", "error", err.Error())
			return "", err
		}
	}
	redirectURI, err := self.RedirectURI(request)
	if err != nil {
		slog.Debug("fnnas oauth authorize gateway failed", "reason", "redirect uri invalid", "error", err.Error())
		return "", err
	}
	state := uuid.NewString()
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyOauthState, state), redirectURI, fnnasStateTTL)

	code := uuid.NewString()
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyOauthCode, code), &FnnasCode{
		Code:        code,
		Provider:    ProviderFnnas,
		RedirectURI: redirectURI,
		State:       state,
		User:        fnnasUser,
		ExpiresAt:   time.Now().Add(fnnasCodeTTL),
	}, fnnasCodeTTL)

	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	query := redirectURL.Query()
	query.Set("code", code)
	query.Set("state", state)
	redirectURL.RawQuery = query.Encode()
	slog.Debug("fnnas oauth authorize gateway success",
		"callbackHost", redirectURL.Hostname(),
		"callbackPort", redirectURL.Port(),
	)
	return redirectURL.String(), nil
}

func (self Fnnas) HasUserHeader(request *http.Request) bool {
	return request.Header.Get("X-Trim-Userid") != "" ||
		request.Header.Get("X-Trim-Isadmin") != "" ||
		request.Header.Get("X-Trim-Username") != ""
}

func (self Fnnas) User(request *http.Request) (FnnasUser, error) {
	userID := request.Header.Get("X-Trim-Userid")
	isAdmin := request.Header.Get("X-Trim-Isadmin")
	username := request.Header.Get("X-Trim-Username")
	if userID == "" || isAdmin == "" || username == "" {
		return FnnasUser{}, errors.New("fnnas user header is empty")
	}
	return FnnasUser{
		UserID:   userID,
		Username: username,
		IsAdmin:  strings.EqualFold(strings.TrimSpace(isAdmin), "true"),
	}, nil
}

func (self Fnnas) ValidateStateRedirect(state string, redirectURI string) error {
	if state == "" {
		return errors.New("oauth state is empty")
	}
	if redirectURI == "" {
		return errors.New("oauth redirect uri is empty")
	}
	if _, err := self.ParseRedirectURI(redirectURI); err != nil {
		return err
	}
	item, exists := storage.Cache.Get(fmt.Sprintf(storage.CacheKeyOauthState, state))
	if !exists {
		return errors.New("oauth state is invalid")
	}
	if item.(string) != redirectURI {
		return errors.New("oauth redirect uri is invalid")
	}
	return nil
}

func (self Fnnas) ParseRedirectURI(redirectURI string) (*url.URL, error) {
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		return nil, err
	}
	if redirectURL.Scheme != "http" && redirectURL.Scheme != "https" {
		return nil, errors.New("oauth redirect uri scheme is invalid")
	}
	if redirectURL.Host == "" {
		return nil, errors.New("oauth redirect uri host is empty")
	}
	if redirectURL.Path != function.RouterUri("/dpanel/ui/user/oauth/callback/fnnas") {
		return nil, errors.New("oauth redirect uri path is invalid")
	}
	if redirectURL.RawQuery != "" || redirectURL.Fragment != "" {
		return nil, errors.New("oauth redirect uri is invalid")
	}
	return redirectURL, nil
}

func (self Fnnas) RedirectURI(request *http.Request) (string, error) {
	host := strings.TrimSpace(request.Host)
	if forwardedHost := request.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		host = strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	if host == "" {
		return "", errors.New("oauth redirect uri host is empty")
	}
	hostname := host
	if value, _, err := net.SplitHostPort(host); err == nil {
		hostname = value
	}
	appPort := ""
	if facade.Config != nil {
		appPort = strings.TrimSpace(facade.Config.GetString("server.http.port"))
	}
	if appPort == "" {
		appPort = strings.TrimSpace(os.Getenv("APP_SERVER_PORT"))
	}
	if appPort == "" {
		return "", errors.New("oauth redirect uri app port is empty")
	}
	host = net.JoinHostPort(strings.Trim(hostname, "[]"), appPort)
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := request.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	}
	if scheme != "http" && scheme != "https" {
		return "", errors.New("oauth redirect uri scheme is invalid")
	}
	return scheme + "://" + host + function.RouterUri("/dpanel/ui/user/oauth/callback/fnnas"), nil
}

func (self Fnnas) IsTcpRequest(request *http.Request) bool {
	_, _, err := net.SplitHostPort(request.RemoteAddr)
	return err == nil
}
