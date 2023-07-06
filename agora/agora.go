package agora

import (
	"errors"
	"fmt"
	"gyrotools/gtagora-connector-go/internals/http"
	"gyrotools/gtagora-connector-go/internals/utils"
)

type Agora struct {
	Client *http.Client
}

func NewAgora(url string, apiKey string, verifyCert bool) *Agora {
	return &Agora{Client: http.NewClient(url, apiKey, verifyCert)}
}

func Create(url string, apiKey string, verifyCertificate bool) (*Agora, error) {
	url, err := utils.ValidateURL(url)
	if err != nil {
		return nil, errors.New("invalid url")
	}
	agora := NewAgora(url, apiKey, verifyCertificate)

	err = agora.Client.CheckConnection()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("cannot connect to Agora: %s", err.Error()))
	}
	return agora, nil
}

func GetApiKey(url string, username string, password string, verifyCertificate bool) (string, error) {
	client := http.NewPasswordClient(url, username, password, verifyCertificate)
	return client.GetApiKey()
}
