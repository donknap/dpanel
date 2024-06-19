package acme

import "fmt"

type NginxProvider struct {
}

func NewNginxProvider() *NginxProvider {
	return &NginxProvider{}
}

func (self NginxProvider) Present(domain, token, keyAuth string) error {
	fmt.Printf("%v \n", domain)
	fmt.Printf("%v \n", token)
	fmt.Printf("%v \n", keyAuth)
	return nil
}

func (self NginxProvider) CleanUp(domain, token, keyAuth string) error {
	return nil
}
