package models

import (
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
