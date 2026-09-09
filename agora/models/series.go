package models

import (
	"fmt"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const SeriesURL = "api/v2/series/"

type Series struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Exam *int   `json:"exam"`

	http.BaseModel
}

func (s *Series) GetDatasets() ([]Dataset, error) {
	path := fmt.Sprintf("%s%d/datasets/?limit=10000000000", SeriesURL, s.ID)
	var datasets []Dataset
	err := s.Client.GetAndParse(path, &datasets)
	if err != nil {
		return nil, err
	}
	return datasets, nil
}
