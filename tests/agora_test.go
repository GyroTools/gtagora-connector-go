package test

import (
	"os"
	"testing"

	"github.com/GyroTools/gtagora-connector-go/agora"
	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

func TestPing(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := "https://gauss4.ethz.ch"
	client := http.NewClient(url, apiKey, false)
	err := client.Ping()
	if err != nil {
		t.Errorf("could not ping Agora: %s", err.Error())
	}
}

func TestConnect(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := "https://gauss4.ethz.ch"
	_, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "https://gauss4.ethz.ch/api/v2/project/"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "gauss4.ethz.ch"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "gauss4.ethz.ch/api/v2/project/"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}
}

func TestConnectWithPassword(t *testing.T) {
	username := os.Getenv("AGORA_USERNAME")
	if len(username) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_USERNAME")
		return
	}
	password := os.Getenv("AGORA_PASSWORD")
	if len(password) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_PASSWORD")
		return
	}

	url := "https://gauss4.ethz.ch"
	_, err := agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "https://gauss4.ethz.ch/api/v2/project/"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "gauss4.ethz.ch"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "gauss4.ethz.ch/api/v2/project/"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}
}

func TestGetApiKey(t *testing.T) {
	username := os.Getenv("AGORA_USERNAME")
	if len(username) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_USERNAME")
		return
	}
	password := os.Getenv("AGORA_PASSWORD")
	if len(password) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_PASSWORD")
		return
	}

	url := "https://gauss4.ethz.ch"
	passwordClient := http.NewPasswordClient(url, username, password, false)
	apiKey, err := passwordClient.GetApiKey()
	if err != nil {
		t.Errorf("could not get the api key: %s", err.Error())
	} else if len(apiKey) == 0 {
		t.Errorf("api key is empty")
	}
}

func TestGetProjects(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := "https://gauss4.ethz.ch"
	agora, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	project, err := agora.GetProject(21)
	if err != nil {
		t.Errorf("cannot get the project: %s", err.Error())
	} else if project == nil {
		t.Errorf("project is empty")
	}

	projects, err := agora.GetProjects()
	if err != nil {
		t.Errorf("cannot get the projects: %s", err.Error())
	} else if len(projects) == 0 {
		t.Errorf("projects is empty")
	}
}
