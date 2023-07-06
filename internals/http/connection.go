package http

import "encoding/base64"

type Auth struct {
	Key   string
	Value string
}

type Connection interface {
	auth() *Auth
	getUrl() string
	verifyCertificate() bool
}

type ApiKeyConnection struct {
	url        string
	verifyCert bool
	apiKey     string
}

func (c *ApiKeyConnection) auth() *Auth {
	if len(c.apiKey) > 0 {
		key := "Authorization"
		value := "X-Agora-Api-Key " + c.apiKey
		return &Auth{Key: key, Value: value}
	}
	return nil
}

func (c *ApiKeyConnection) getUrl() string {
	return c.url
}

func (c *ApiKeyConnection) verifyCertificate() bool {
	return c.verifyCert
}

type PasswordConnection struct {
	url        string
	verifyCert bool
	username   string
	password   string
}

func (c PasswordConnection) basicAuth(username string, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (c PasswordConnection) auth() *Auth {
	if len(c.username) > 0 && len(c.password) > 0 {
		key := "Authorization"
		value := "Basic " + c.basicAuth(c.username, c.password)
		return &Auth{Key: key, Value: value}
	}
	return nil
}

func (c PasswordConnection) getUrl() string {
	return c.url
}

func (c PasswordConnection) verifyCertificate() bool {
	return c.verifyCert
}
