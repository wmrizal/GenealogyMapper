package graph

import (
	"context"
	"errors"
	"fmt"

	"genealogymapper/internal/domain"
)

var (
	ErrPersonNotFound = errors.New("person not found")
	ErrPersonExists   = errors.New("person already exists")
	ErrRelationExists = errors.New("parent-child relationship already exists")
)

type Repository interface {
	CreatePerson(ctx context.Context, person domain.Person) (domain.Person, error)
	UpdatePerson(ctx context.Context, person domain.Person) (domain.Person, error)
	CreatePersonAndLink(ctx context.Context, person domain.Person, relation domain.ParentChild) (domain.Person, error)
	GetPerson(ctx context.Context, personID string) (domain.Person, error)
	SearchPeople(ctx context.Context, query string) ([]domain.Person, error)
	AddParentChild(ctx context.Context, relation domain.ParentChild) (created bool, err error)
	GetSnapshot(ctx context.Context) (domain.GraphSnapshot, error)
	ReplaceSnapshot(ctx context.Context, snapshot domain.GraphSnapshot) error
	GetNeighborhood(ctx context.Context, personID string, maxHops int) (domain.GraphSnapshot, error)
}

func ValidateSnapshot(snapshot domain.GraphSnapshot) error {
	peopleByID := make(map[string]struct{}, len(snapshot.People))
	for _, person := range snapshot.People {
		if err := person.Validate(); err != nil {
			return fmt.Errorf("validate person %q: %w", person.ID, err)
		}
		if _, exists := peopleByID[person.ID]; exists {
			return fmt.Errorf("duplicate person ID %q", person.ID)
		}
		peopleByID[person.ID] = struct{}{}
	}

	relations := make(map[domain.ParentChild]struct{}, len(snapshot.ParentChild))
	for _, relation := range snapshot.ParentChild {
		if err := relation.Validate(); err != nil {
			return err
		}
		if _, exists := peopleByID[relation.ParentID]; !exists {
			return fmt.Errorf("relationship parent %q was not found", relation.ParentID)
		}
		if _, exists := peopleByID[relation.ChildID]; !exists {
			return fmt.Errorf("relationship child %q was not found", relation.ChildID)
		}
		if _, exists := relations[relation]; exists {
			return fmt.Errorf("duplicate relationship %q -> %q", relation.ParentID, relation.ChildID)
		}
		relations[relation] = struct{}{}
	}
	return nil
}
