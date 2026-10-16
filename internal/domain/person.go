package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrPersonIDRequired = errors.New("person id is required")
	ErrNameRequired     = errors.New("person name is required")
	ErrInvalidDateRange = errors.New("death date cannot be before birth date")
)

type Person struct {
	ID        string
	Name      string
	BirthDate *time.Time
	DeathDate *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewPerson(id, name string, birthDate, deathDate *time.Time, now time.Time) (Person, error) {
	person := Person{
		ID:        strings.TrimSpace(id),
		Name:      strings.TrimSpace(name),
		BirthDate: birthDate,
		DeathDate: deathDate,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := person.Validate(); err != nil {
		return Person{}, err
	}
	return person, nil
}

func (p Person) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return ErrPersonIDRequired
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrNameRequired
	}
	if p.BirthDate != nil && p.DeathDate != nil && p.DeathDate.Before(*p.BirthDate) {
		return ErrInvalidDateRange
	}
	return nil
}
