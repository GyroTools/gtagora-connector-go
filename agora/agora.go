package agora

import (
	"errors"
	"fmt"

	"github.com/GyroTools/gtagora-connector-go/agora/models"
	"github.com/GyroTools/gtagora-connector-go/internals/http"
	"github.com/GyroTools/gtagora-connector-go/internals/utils"
)

type Agora struct {
	Client *http.Client
}

func Ping(url string) error {
	client := http.NewClient(url, "", false)
	return client.Ping()
}

func (a *Agora) GetApiKey() (string, error) {
	return a.Client.GetApiKey()
}

func (a *Agora) GetProjects() ([]models.Project, error) {
	var projects []models.Project
	err := a.Client.GetAndParse(models.ProjectURL, &projects)
	if err != nil {
		return nil, err
	}
	return projects, nil
}

func (a *Agora) GetProject(id int) (*models.Project, error) {
	var project models.Project

	err := a.Client.GetAndParse(fmt.Sprintf("%s%d/", models.ProjectURL, id), &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (a *Agora) GetFolder(id int) (*models.Folder, error) {
	var folder models.Folder

	err := a.Client.GetAndParse(fmt.Sprintf("%s%d/", models.FolderURL, id), &folder)
	if err != nil {
		return nil, err
	}
	return &folder, nil
}

func (a *Agora) GetFolderItem(id int) (*models.FolderItem, error) {
	var folderItem models.FolderItem

	err := a.Client.GetAndParse(fmt.Sprintf("%s%d/", models.FolderItemURL, id), &folderItem)
	if err != nil {
		return nil, err
	}
	return &folderItem, nil
}

func (a *Agora) NewImportPackage() (*models.ImportPackage, error) {
	var importPackage models.ImportPackage

	err := a.Client.PostAndParse(fmt.Sprintf("%s", models.ImportPackageURL), nil, &importPackage)
	if err != nil {
		return nil, err
	}
	return &importPackage, nil
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

func CreateWithPassword(url string, username string, password string, verifyCertificate bool) (*Agora, error) {
	url, err := utils.ValidateURL(url)
	if err != nil {
		return nil, errors.New("invalid url")
	}
	passwordClient := http.NewPasswordClient(url, username, password, verifyCertificate)
	apiKey, err := passwordClient.GetApiKey()
	if err != nil {
		return nil, err
	}

	agora := NewAgora(url, apiKey, verifyCertificate)

	err = agora.Client.CheckConnection()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("cannot connect to Agora: %s", err.Error()))
	}
	return agora, nil
}
