package models

import (
	"fmt"
	"time"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const StudyURL = "api/v2/exam/"

type Study struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Project     *int      `json:"project"`
	Patient     *Patient  `json:"patient"`
	Uid         string    `json:"uid"`
	ScannerName string    `json:"scanner_name"`
	Vendor      int       `json:"vendor"`
	StartTime   time.Time `json:"start_time"`
	CreatedDate time.Time `json:"created_date"`

	http.BaseModel
}

// GetSeries returns the series belonging to this exam.
func (s *Study) GetSeries() ([]Series, error) {
	path := fmt.Sprintf("%s%d/series/?limit=10000000000", StudyURL, s.ID)
	var series []Series
	err := s.Client.GetAndParse(path, &series)
	if err != nil {
		return nil, err
	}
	return series, nil
}

// GetDirectDatasets returns the datasets attached directly to this exam
// (i.e. not attached via one of its series).
func (s *Study) GetDirectDatasets() ([]Dataset, error) {
	path := fmt.Sprintf("%s%d/datasets/?limit=10000000000", StudyURL, s.ID)
	var datasets []Dataset
	err := s.Client.GetAndParse(path, &datasets)
	if err != nil {
		return nil, err
	}
	return datasets, nil
}
