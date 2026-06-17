package oauth

import (
	"fmt"
	"net/http"
)

const ProviderFnnas = "fnnas"

type Provider interface {
	Item() (Item, bool)
	Authorize(request *http.Request) (string, error)
	Exchange(option ExchangeOption) (string, error)
}

type Item struct {
	Provider     string `json:"provider"`
	Name         string `json:"name"`
	AuthorizeURL string `json:"authorizeUrl"`
}

type ExchangeOption struct {
	Code        string
	State       string
	RedirectURI string
}

func Authorize(provider string, request *http.Request) (string, error) {
	providerIns, err := providerByName(provider)
	if err != nil {
		return "", err
	}
	return providerIns.Authorize(request)
}

func Exchange(provider string, option ExchangeOption) (string, error) {
	providerIns, err := providerByName(provider)
	if err != nil {
		return "", err
	}
	return providerIns.Exchange(option)
}

func providerByName(provider string) (Provider, error) {
	switch provider {
	case ProviderFnnas:
		return Fnnas{}, nil
	default:
		return nil, fmt.Errorf("unsupported oauth provider: %s", provider)
	}
}
