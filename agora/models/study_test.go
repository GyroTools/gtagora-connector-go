package models

import (
	"encoding/json"
	"testing"
)

// The API represents Study's "patient" field as a full embedded Patient
// object on some endpoints and as a bare patient id on others (e.g. an
// exam summary embedded in a folder's items listing) - Study.Patient must
// decode both shapes instead of crashing.
func TestStudyPatientUnmarshalsBothShapes(t *testing.T) {
	var withID Study
	if err := json.Unmarshal([]byte(`{"id":100,"name":"Exam","patient":42}`), &withID); err != nil {
		t.Fatalf("unmarshal with bare patient id failed: %s", err)
	}
	if withID.Patient == nil {
		t.Fatalf("expected Patient to be non-nil")
	}
	if withID.Patient.ID != 42 {
		t.Errorf("got id %d, want 42", withID.Patient.ID)
	}
	if withID.Patient.Patient != nil {
		t.Errorf("expected embedded Patient object to be nil when only an id was given")
	}

	var withObject Study
	if err := json.Unmarshal([]byte(`{"id":100,"name":"Exam","patient":{"id":42,"name":"Doe, John"}}`), &withObject); err != nil {
		t.Fatalf("unmarshal with embedded patient object failed: %s", err)
	}
	if withObject.Patient == nil || withObject.Patient.Patient == nil {
		t.Fatalf("expected an embedded Patient object")
	}
	if withObject.Patient.ID != 42 {
		t.Errorf("got id %d, want 42", withObject.Patient.ID)
	}
	if withObject.Patient.Patient.Name != "Doe, John" {
		t.Errorf("got name %q, want %q", withObject.Patient.Patient.Name, "Doe, John")
	}

	var withNull Study
	if err := json.Unmarshal([]byte(`{"id":100,"name":"Exam","patient":null}`), &withNull); err != nil {
		t.Fatalf("unmarshal with null patient failed: %s", err)
	}
	if withNull.Patient != nil {
		t.Errorf("expected Patient to be nil for a null patient field")
	}
}
