package http

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
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

// this is defined in the client because other wise we would have a circular import:
// BaseModel needs to import client and client needs to import BaseModel
type BaseModel struct {
	URL    string  `json:"-"`
	Client *Client `json:"-"`
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
	resp, err := client.Get("/api/v1/version/", time.Duration(5*time.Second))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}
	return nil
}

func (client *Client) CheckConnection() error {
	resp, err := client.Get("/api/v1/user/current/", time.Duration(5*time.Second))
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}
	return nil
}

func (client *Client) GetApiKey() (string, error) {
	if apiConn, ok := client.conn.(*ApiKeyConnection); ok {
		return apiConn.apiKey, nil
	} else if _, ok := client.conn.(*PasswordConnection); ok {

		err := client.Ping()
		if err != nil {
			return "", err
		}
		resp, err := client.Get("/api/v1/apikey/", -1)

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
	return "", errors.New("unknown connection")
}

func (client *Client) GetAndParse(path string, target interface{}) error {
	resp, err := client.Get(path, -1)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return client.parseResponse(resp, target, path)
}

func (client *Client) PostAndParse(path string, body io.Reader, target interface{}) error {
	resp, err := client.Post(path, body, -1)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return client.parseResponse(resp, target, path)
}

func (client *Client) parseResponse(resp *http.Response, target interface{}, path string) error {
	if resp.StatusCode >= 400 {
		return errors.New(fmt.Sprintf("status code = %d", resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, &target)
	if err != nil {
		return err
	}

	targetType := reflect.TypeOf(target)
	targetValue := reflect.ValueOf(target)

	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
		targetValue = targetValue.Elem()
	}

	if targetType.Kind() == reflect.Slice {
		for i := 0; i < targetValue.Len(); i++ {
			elem := targetValue.Index(i)
			if baseModel, ok := getBaseModelFromStruct(elem); ok {
				baseModel.URL = path
				baseModel.Client = client
			}
		}
	} else {
		if baseModel, ok := getBaseModelFromStruct(targetValue); ok {
			baseModel.URL = path
			baseModel.Client = client
		}
	}

	return nil
}

func getBaseModelFromStruct(value reflect.Value) (*BaseModel, bool) {
	n := value.NumField()
	for i := 0; i < n; i++ {
		field := value.Field(i)
		if field.Type().Name() == "BaseModel" {
			return field.Addr().Interface().(*BaseModel), true
		}
	}
	return nil, false
}

func (client Client) GetUrl(path string) string {
	u, err := url.Parse(client.conn.getUrl())
	if err != nil {
		return client.conn.getUrl() + path
	}
	// Parse the provided path separately
	parsedPath, err := url.Parse(path)
	if err != nil {
		return client.conn.getUrl() + path
	}

	// Resolve the parsed path against the base URL
	resolvedURL := u.ResolveReference(parsedPath).String()
	return resolvedURL
}

func handleNoCertificateCheck(check bool) {
	if !check {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}

func (client *Client) Get(path string, timeout time.Duration) (*http.Response, error) {
	if client.conn != nil {
		handleNoCertificateCheck(client.conn.verifyCertificate())
	}
	url := client.GetUrl(path)
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

	if client.conn != nil {
		auth := client.conn.auth()
		if auth != nil {
			req.Header.Set(auth.Key, auth.Value)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (client *Client) Post(path string, body io.Reader, timeout time.Duration) (*http.Response, error) {
	if client.conn != nil {
		handleNoCertificateCheck(client.conn.verifyCertificate())
	}
	url := client.GetUrl(path)
	if timeout == -1 {
		timeout = DefaultTimeout * time.Second // Set the default timeout duration
	}
	httpClient := &http.Client{
		Timeout: timeout,
	}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	if client.conn != nil {
		auth := client.conn.auth()
		if auth != nil {
			req.Header.Set(auth.Key, auth.Value)
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, err
}
