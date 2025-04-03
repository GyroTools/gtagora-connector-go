package models

import (
	"fmt"
	"time"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const ProjectURL = "api/v2/project/"

type Project struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Description  *string      `json:"description"`
	Memberships  []Membership `json:"memberships"`
	RootFolder   int          `json:"root_folder"`
	Owner        *int         `json:"owner"`
	IsMyAgora    bool         `json:"is_myagora"`
	SpecialFunc  *string      `json:"special_function"`
	AnonSettings *int         `json:"anon_settings"`
	AnonProfile  *int         `json:"anon_profile_set"`
	CreatedDate  time.Time    `json:"created_date"`

	http.BaseModel
}

type Membership struct {
	User    int `json:"user"`
	Role    int `json:"role"`
	ID      int `json:"id"`
	Project int `json:"project"`
}

func (project *Project) GetStudies() ([]Study, error) {
	var studies []Study

	url := fmt.Sprintf("%sexam/?limit=10000000000", project.URL)
	err := project.Client.GetAndParse(url, &studies)
	if err != nil {
		return nil, err
	}
	return studies, nil
}

func (project *Project) GetPatients() ([]Patient, error) {
	var patients []Patient

	url := fmt.Sprintf("%spatient/?limit=10000000000", project.URL)
	err := project.Client.GetAndParse(url, &patients)
	if err != nil {
		return nil, err
	}
	return patients, nil
}
