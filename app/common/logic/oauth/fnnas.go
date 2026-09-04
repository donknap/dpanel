package oauth

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	commonLogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
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
		return "", errors.New("oauth code is invalid")
	}
	codeInfo, ok := item.(*FnnasCode)
	if !ok || codeInfo.Provider != ProviderFnnas {
		return "", errors.New("oauth code is invalid")
	}
	if codeInfo.Used {
		return "", errors.New("oauth code has been used")
	}
	if time.Now().After(codeInfo.ExpiresAt) {
		storage.Cache.Delete(cacheKey)
		return "", errors.New("oauth code has expired")
	}
	if option.State == "" || codeInfo.State != option.State {
		return "", errors.New("oauth state is invalid")
	}
	if option.RedirectURI != "" && codeInfo.RedirectURI != option.RedirectURI {
		return "", errors.New("oauth redirect uri is invalid")
	}
	if !codeInfo.User.IsAdmin {
		codeInfo.Used = true
		storage.Cache.Set(cacheKey, codeInfo, time.Second)
		storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyOauthState, codeInfo.State))
		return "", function.ErrorMessage(define.ErrorMessageOauthAdminRequired)
	}

	founder, err := commonLogic.User{}.GetFounderUser()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if founder, err = (commonLogic.User{}).CreateFounderUser(codeInfo.User.Username, uuid.NewString()); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	codeInfo.Used = true
	storage.Cache.Set(cacheKey, codeInfo, time.Second)
	storage.Cache.Delete(fmt.Sprintf(storage.CacheKeyOauthState, codeInfo.State))
	accessToken, err := commonLogic.User{}.GetUserOauthToken(founder, false)
	if err != nil {
		return "", err
	}
	return accessToken, nil
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
		return "", err
	}
	redirectURI, err := self.RedirectURI(request)
	if err != nil {
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
	return redirectURL.String(), nil
}

func (self Fnnas) HasUserHeader(request *http.Request) bool {
	return request.Header.Get("X-Trim-Userid") != "" ||
		request.Header.Get("X-Trim-Isadmin") != "" ||
		request.Header.Get("X-Trim-Username") != ""
}

func (self Fnnas) User(request *http.Request) (FnnasUser, error) {
	userID := request.Header.Get("X-Trim-Userid")
	isAdminRaw := request.Header.Get("X-Trim-Isadmin")
	username := request.Header.Get("X-Trim-Username")
	parsedAdmin, parseErr := strconv.ParseBool(strings.TrimSpace(isAdminRaw))
	if userID == "" || isAdminRaw == "" || username == "" {
		return FnnasUser{}, errors.New("fnnas user header is empty")
	}
	if parseErr != nil {
		return FnnasUser{}, errors.New("fnnas user isadmin header is invalid")
	}
	return FnnasUser{
		UserID:   userID,
		Username: username,
		IsAdmin:  parsedAdmin,
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

	// Preserve the forwarded Host, including its port, and apply the panel
	// baseurl exactly once to the callback path.
	return scheme + "://" + host + function.RouterUri("/dpanel/ui/user/oauth/callback/fnnas"), nil
}

func (self Fnnas) IsTcpRequest(request *http.Request) bool {
	_, _, err := net.SplitHostPort(request.RemoteAddr)
	return err == nil
}
