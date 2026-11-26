package oauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	commonLogic "github.com/donknap/dpanel/app/common/logic"
	"github.com/donknap/dpanel/common/function"
	"github.com/donknap/dpanel/common/service/exec/local"
	"github.com/donknap/dpanel/common/service/storage"
	"github.com/donknap/dpanel/common/types/define"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Synology struct{}

type synologyGrant struct {
	Username string
	State    string
}

const synologyCallbackPath = "/dpanel/ui/user/oauth/callback/synology"

var synologyExchangeLock sync.Mutex

// 限制公开授权入口同时启动的 CGI 数量。
var synologyAuthSlot = make(chan struct{}, 4)

func (Synology) Item() (Item, bool) { return Item{}, false }

func (self Synology) Authorize(request *http.Request) (string, error) {
	username, err := self.adminUsername(request)
	if err != nil {
		slog.Info("synology login denied", "reason", err)
		return function.RouterUri("/dpanel/ui/user/login"), nil
	}
	code := uuid.NewString()
	state := uuid.NewString()
	storage.Cache.Set(fmt.Sprintf(storage.CacheKeyOauthCode, code), synologyGrant{Username: username, State: state}, time.Minute)
	callback := url.URL{Path: function.RouterUri(synologyCallbackPath), RawQuery: url.Values{"code": {code}, "state": {state}}.Encode()}
	return callback.String(), nil
}

func (Synology) adminUsername(request *http.Request) (string, error) {
	if runtime.GOOS != "linux" || os.Getenv("SYNOPKG_PKGNAME") != "dpanel" {
		return "", errors.New("not_synology_package")
	}
	// 桌面入口是浏览器导航，不接受跨站页面代发的授权请求。
	if request.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return "", errors.New("cross_site")
	}
	if mode := request.Header.Get("Sec-Fetch-Mode"); mode != "" && mode != "navigate" {
		return "", errors.New("not_navigation")
	}
	cookie := strings.Join(request.Header.Values("Cookie"), "; ")
	if cookie == "" {
		return "", errors.New("missing_cookie")
	}
	remote, _, err := net.SplitHostPort(request.RemoteAddr)
	localAddr, ok := request.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr)
	if err != nil || net.ParseIP(remote) == nil || !ok || localAddr.IP == nil {
		return "", errors.New("connection_address")
	}
	// 只传 CGI 所需的上下文，地址取自连接而非可伪造的转发头。
	env := []string{
		"HTTP_COOKIE=" + cookie, "REMOTE_ADDR=" + remote, "SERVER_ADDR=" + localAddr.IP.String(),
		"SERVER_PORT=" + strconv.Itoa(localAddr.Port), "REQUEST_METHOD=" + request.Method,
		"HTTP_USER_AGENT=" + request.UserAgent(), "SERVER_PROTOCOL=" + request.Proto,
	}
	if request.TLS != nil {
		env = append(env, "HTTPS=on")
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return "", errors.New("invalid_synology_query")
	}
	if tokenList, ok := query["k"]; ok {
		// 严格 Base64 解码仍会忽略换行，必须提前拒绝。
		if len(tokenList) != 1 || tokenList[0] == "" || strings.ContainsAny(tokenList[0], "\r\n") {
			return "", errors.New("invalid_synology_token")
		}
		tokenBytes, err := base64.RawURLEncoding.Strict().DecodeString(tokenList[0])
		token := string(tokenBytes)
		if err != nil || !utf8.ValidString(token) || strings.IndexFunc(token, unicode.IsControl) >= 0 {
			return "", errors.New("invalid_synology_token")
		}
		env = append(env, "HTTP_X_SYNO_TOKEN="+token)
	}
	select {
	case synologyAuthSlot <- struct{}{}:
		defer func() { <-synologyAuthSlot }()
	default:
		return "", errors.New("busy")
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	output, err := local.RunWithStdout(
		local.WithCommandName("/usr/syno/synoman/webman/modules/authenticate.cgi"),
		local.WithEnv(env),
		local.WithQuiet(),
		local.WithCtx(ctx),
		local.WithIndependentProcessGroup(),
		local.WithKillProcessGroupOnCancel(),
	)
	if err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("synology authentication: %w", ctx.Err())
		}
		// 不回显 CGI 的 stdout/stderr，其中可能包含会话信息。
		return "", fmt.Errorf("synology authentication: %w", err)
	}
	username := strings.TrimSuffix(strings.TrimSuffix(string(output), "\n"), "\r")
	if username == "" || strings.IndexFunc(username, unicode.IsControl) >= 0 {
		return "", errors.New("unverified_session")
	}
	account, err := user.Lookup(username)
	if err != nil {
		return "", errors.New("account_lookup")
	}
	group, err := user.LookupGroup("administrators")
	if err != nil {
		return "", errors.New("admin_group_lookup")
	}
	groupList, err := account.GroupIds()
	if err != nil {
		return "", errors.New("group_lookup")
	}
	if !slices.Contains(groupList, group.Gid) {
		return "", errors.New("not_administrator")
	}
	return username, nil
}

func (Synology) Exchange(option ExchangeOption) (string, error) {
	synologyExchangeLock.Lock()
	defer synologyExchangeLock.Unlock()
	key := fmt.Sprintf(storage.CacheKeyOauthCode, option.Code)
	item, _ := storage.Cache.Get(key)
	grant, ok := item.(synologyGrant)
	if !ok || option.State != grant.State || (option.RedirectURI != "" && option.RedirectURI != function.RouterUri(synologyCallbackPath)) {
		slog.Info("synology login denied", "reason", "invalid_grant")
		return "", function.ErrorMessage(define.ErrorMessageUserLogin)
	}
	storage.Cache.Delete(key)
	founder, err := commonLogic.User{}.GetFounderUser()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		founder, err = (commonLogic.User{}).CreateFounderUser(grant.Username, uuid.NewString())
	}
	if err != nil {
		return "", err
	}
	token, err := commonLogic.User{}.GetUserOauthToken(founder, false)
	if err != nil {
		return "", err
	}
	slog.Info("synology login succeeded")
	return token, nil
}
