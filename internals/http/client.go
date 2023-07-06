package http

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultTimeout = 10
)

type Client struct {
	conn Connection
}

type ApiKeyResponse struct {
	ApiKey string `json:"key"`
}

func NewClient(url string, apiKey string, verifyCert bool) *Client {
	connection := &ApiKeyConnection{verifyCert: verifyCert, apiKey: apiKey, url: url}
	return &Client{conn: connection}
}

func NewPasswordClient(url string, username string, password string, verifyCert bool) *Client {
	connection := PasswordConnection{verifyCert: verifyCert, username: username, password: password, url: url}
	return &Client{conn: &connection}
}

func (client *Client) Ping() error {
	resp, err := client.get("/api/v1/version/", time.Duration(5*time.Second))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}
	return nil
}

func (client *Client) CheckConnection() error {
	resp, err := client.get("/api/v1/user/current/", time.Duration(5*time.Second))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}
	return nil
}

func (client *Client) GetApiKey() (string, error) {
	_, ok := client.conn.(*PasswordConnection)
	if !ok {
		return "", errors.New("you must connect with username/password to get the api key")
	}
	err := client.Ping()
	if err != nil {
		return "", err
	}
	resp, err := client.get("/api/v1/apikey/", -1)

	if err != nil {
		return "", nil
	}
	if resp.StatusCode == 404 {
		return "", errors.New("no api-key found. please create an api-key in your Agora user profile")
	} else if resp.StatusCode > 299 {
		return "", errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}

	target := new(ApiKeyResponse)
	json.NewDecoder(resp.Body).Decode(target)
	return target.ApiKey, nil
}

func (client Client) getUrl(path string) string {
	u, err := url.Parse(client.conn.getUrl())
	if err != nil {
		return client.conn.getUrl() + path
	}
	u.Path = path
	return u.String()
}

func handleNoCertificateCheck(check bool) {
	if !check {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

func (client *Client) get(path string, timeout time.Duration) (*http.Response, error) {
	handleNoCertificateCheck(client.conn.verifyCertificate())
	url := client.getUrl(path)
	if timeout == -1 {
		timeout = DefaultTimeout * time.Second // Set the default timeout duration
	}
	httpClient := &http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	auth := client.conn.auth()
	if auth != nil {
		req.Header.Set(auth.Key, auth.Value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}
