package models

import (
	"fmt"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const DatasetURL = "api/v2/dataset/"

type Dataset struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	TotalSize int64  `json:"total_size"`

	http.BaseModel
}

func (d *Dataset) GetDatafiles() ([]Datafile, error) {
	path := fmt.Sprintf("%s%d/datafiles/?limit=10000000000", DatasetURL, d.ID)
	var datafiles []Datafile
	err := d.Client.GetAndParse(path, &datafiles)
	if err != nil {
		return nil, err
	}
	return datafiles, nil
}
