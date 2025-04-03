package models

import (
	"time"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const PatientURL = "api/v2/patient/"

type Patient struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	PatientId    *string   `json:"patient_id"`
	BirthDate    *string   `json:"birth_date"`
	Sex          *string   `json:"sex"`
	Weight       *float32  `json:"weight"`
	Anonymous    bool      `json:"anonymous"`
	IsAnonymized bool      `json:"is_anonymized"`
	IsMain       bool      `json:"is_main"`
	CreatedDate  time.Time `json:"created_date"`

	http.BaseModel
}
