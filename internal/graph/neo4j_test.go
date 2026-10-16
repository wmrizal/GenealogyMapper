package graph

import (
	"testing"
	"time"

	"genealogymapper/internal/domain"
)

func TestPersonParametersRoundTripValues(t *testing.T) {
	birthDate := time.Date(1985, time.April, 12, 0, 0, 0, 0, time.UTC)
	person := domain.Person{
		ID:        "person-1",
		Name:      "Example Person",
		BirthDate: &birthDate,
		CreatedAt: time.Date(2026, time.January, 2, 3, 4, 5, 6, time.UTC),
		UpdatedAt: time.Date(2026, time.January, 2, 3, 4, 6, 7, time.UTC),
	}

	parameters := personParameters(person)
	if parameters["birthDate"] != "1985-04-12" {
		t.Fatalf("birthDate parameter = %#v", parameters["birthDate"])
	}
	if parameters["createdAt"] != "2026-01-02T03:04:05.000000006Z" {
		t.Fatalf("createdAt parameter = %#v", parameters["createdAt"])
	}
}

func TestDateAndTimestampProperties(t *testing.T) {
	properties := map[string]any{
		"birthDate": "1985-04-12",
		"createdAt": "2026-01-02T03:04:05Z",
	}

	birthDate, err := dateProperty(properties, "birthDate")
	if err != nil {
		t.Fatalf("dateProperty() error = %v", err)
	}
	if birthDate == nil || birthDate.Format(dateFormat) != "1985-04-12" {
		t.Fatalf("birthDate = %v", birthDate)
	}

	createdAt, err := timestampProperty(properties, "createdAt")
	if err != nil {
		t.Fatalf("timestampProperty() error = %v", err)
	}
	if createdAt.Format(timestampFormat) != "2026-01-02T03:04:05Z" {
		t.Fatalf("createdAt = %v", createdAt)
	}
}
