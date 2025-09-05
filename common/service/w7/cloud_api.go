package w7

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/donknap/dpanel/common/service/w7/cache"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

func NewCloudApi(credential *Credential) *CloudApi {
	return &CloudApi{
		Credential: credential,
		ReqCache:   cache.NewReqCache(credential.Secret, time.Hour*2),
	}
}

type ApiError struct {
	ErrorMsg string `json:"error"`
	Code     int    `json:"code"`
}

func (ve ApiError) Error() string {
	return ve.ErrorMsg
}

type CloudApi struct {
	Credential *Credential
	ReqCache   *cache.ReqCache
}

type OpenSoftwareVerifyReq struct {
	VerifyPayload string `json:"verify_payload"`
	VerifyType    string `json:"verify_type"`
	GoodsSn       string `json:"goods_sn"`
	DeviceSn      string `json:"device_sn"`
	Extra         string `json:"extra"`
}

type OpenSoftwareVerifyResp struct {
	IsValid     bool   `json:"valid"`
	DeviceSn    string `json:"device_sn"`
	Extra       string `json:"extra"`
	IsExpired   bool   `json:"is_expire"`
	ExpireTime  int64  `json:"expire_time"`
	BuyerMobile string `json:"buyer_mobile"`
}

func (s CloudApi) OpenSoftwareLicenseInit(verifyReq OpenSoftwareVerifyReq, ignoreCache bool) (*OpenSoftwareVerifyResp, error) {
	if verifyReq.DeviceSn == "" {
		return nil, fmt.Errorf("device_sn is empty")
	}

	return s.OpenSoftwareLicenseVerify(verifyReq, ignoreCache)
}

func (s CloudApi) OpenSoftwareLicenseVerify(verifyReq OpenSoftwareVerifyReq, ignoreCache bool) (*OpenSoftwareVerifyResp, error) {
	cacheKey := fmt.Sprintf("open_software_license_verify_%s_%s", s.Credential.Appid, verifyReq.VerifyType)
	if !ignoreCache && s.ReqCache != nil {
		var verifyResult *OpenSoftwareVerifyResp
		err := s.ReqCache.Get(cacheKey, &verifyResult)
		if err == nil && verifyResult != nil {
			return verifyResult, nil
		}
	}

	resParams := map[string]string{
		"verify_payload": verifyReq.VerifyPayload,
		"verify_type":    verifyReq.VerifyType,
		"goods_sn":       verifyReq.GoodsSn,
		"device_sn":      verifyReq.DeviceSn,
	}
	if verifyReq.Extra != "" {
		resParams["extra"] = verifyReq.Extra
	}
	reqBody, err := s.buildRequestBody(resParams)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	req, err := http.NewRequest("POST", "https://api.w7.cc/w7api/mgw/w7_opensoftware/verify", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-requested-with", "XMLHttpRequest")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	statusCode := resp.StatusCode
	if statusCode != 200 && statusCode != 201 {
		var apiError ApiError
		err = json.Unmarshal(respBody, &apiError)
		if err != nil {
			return nil, err
		}
		if apiError.ErrorMsg == "" {
			apiError.ErrorMsg = string(respBody)
			apiError.Code = 500
		}
		return nil, apiError
	}

	var openSoftwareVerifyResp *OpenSoftwareVerifyResp
	err = json.Unmarshal(respBody, &openSoftwareVerifyResp)
	if err != nil {
		return nil, err
	}

	if s.ReqCache != nil && openSoftwareVerifyResp.IsValid {
		err = s.ReqCache.Set(cacheKey, openSoftwareVerifyResp)
		if err != nil {
			slog.Error("save req cache err", "req", verifyReq, "resp", openSoftwareVerifyResp, "err", err)
		}
	}

	return openSoftwareVerifyResp, nil
}

func (s CloudApi) buildRequestBody(params map[string]string) ([]byte, error) {
	params["appid"] = s.Credential.Appid
	params["nonce"] = s.makeRandStr(16)
	params["timestamp"] = strconv.FormatInt(time.Now().Unix(), 10)
	params["sign"] = s.getSign(params, s.Credential.Secret)

	formData := url.Values{}
	for key, val := range params {
		formData.Set(key, val)
	}

	return []byte(formData.Encode()), nil
}

func (s CloudApi) getSign(params map[string]string, secret string) string {
	_, ok := params["sign"]
	if ok {
		delete(params, "sign")
	}

	var keys []string
	signStr := ""
	for k, _ := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		signStr += fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params[k]))
		if i < len(keys)-1 {
			signStr += "&"
		}
	}

	signStr += secret
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(signStr))
	return hex.EncodeToString(md5Ctx.Sum(nil))
}

func (s CloudApi) makeRandStr(length int) string {
	str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	bytes := []byte(str)
	result := []byte{}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < length; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}

	return string(result)
}
