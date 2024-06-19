package logic

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/go-acme/lego/v4/registration"
	"github.com/we7coreteam/w7-rangine-go-support/src/facade"
	"os"
)

type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func NewAcmeUser(email string) (*acmeUser, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &acmeUser{
		Email: email,
		key:   privateKey,
	}, nil
}

func (u *acmeUser) GetEmail() string {
	return u.Email
}
func (u acmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

type acmeNginxProvider struct {
}

func NewAcmeNginxProvider() *acmeNginxProvider {
	return &acmeNginxProvider{}
}

func (self acmeNginxProvider) Present(domain, token, keyAuth string) error {
	err := os.WriteFile(self.getChallengeFilePath(token), []byte(keyAuth), 0666)
	if err != nil {
		return err
	}
	fmt.Printf("%v \n", domain)
	fmt.Printf("%v \n", token)
	fmt.Printf("%v \n", keyAuth)
	return nil
}

func (self acmeNginxProvider) CleanUp(domain, token, keyAuth string) error {
	_ = os.Remove(self.getChallengeFilePath(token))
	return nil
}

func (self acmeNginxProvider) getChallengeFilePath(token string) string {
	return fmt.Sprintf("%s/challenge/.well-known/acme-challenge/%s", facade.GetConfig().GetString("storage.local.path"), token)
}
