package genealogy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	"genealogymapper/internal/domain"
	"genealogymapper/internal/graph"
)

const JSONVersion = 1

type jsonDocument struct {
	Version       int                `json:"version"`
	People        []jsonPerson       `json:"people"`
	Relationships []jsonRelationship `json:"relationships"`
}

type jsonPerson struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	BirthDate *string   `json:"birthDate"`
	DeathDate *string   `json:"deathDate"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type jsonRelationship struct {
	ParentID string `json:"parentId"`
	ChildID  string `json:"childId"`
}

func (s *Service) ExportJSON(ctx context.Context) ([]byte, error) {
	snapshot, err := s.repository.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	if err := graph.ValidateSnapshot(snapshot); err != nil {
		return nil, fmt.Errorf("export graph: %w", err)
	}

	people := append([]domain.Person(nil), snapshot.People...)
	sort.Slice(people, func(i, j int) bool {
		if people[i].Name == people[j].Name {
			return people[i].ID < people[j].ID
		}
		return people[i].Name < people[j].Name
	})
	relations := append([]domain.ParentChild(nil), snapshot.ParentChild...)
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].ParentID == relations[j].ParentID {
			return relations[i].ChildID < relations[j].ChildID
		}
		return relations[i].ParentID < relations[j].ParentID
	})

	document := jsonDocument{
		Version:       JSONVersion,
		People:        make([]jsonPerson, 0, len(people)),
		Relationships: make([]jsonRelationship, 0, len(relations)),
	}
	for _, person := range people {
		document.People = append(document.People, jsonPerson{
			ID:        person.ID,
			Name:      person.Name,
			BirthDate: jsonDate(person.BirthDate),
			DeathDate: jsonDate(person.DeathDate),
			CreatedAt: person.CreatedAt,
			UpdatedAt: person.UpdatedAt,
		})
	}
	for _, relation := range relations {
		document.Relationships = append(document.Relationships, jsonRelationship{
			ParentID: relation.ParentID,
			ChildID:  relation.ChildID,
		})
	}

	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode graph JSON: %w", err)
	}
	return append(data, '\n'), nil
}

func (s *Service) ImportJSON(ctx context.Context, data []byte) error {
	snapshot, err := decodeJSON(data)
	if err != nil {
		return err
	}
	if err := graph.ValidateSnapshot(snapshot); err != nil {
		return fmt.Errorf("import graph: %w", err)
	}
	return s.repository.ReplaceSnapshot(ctx, snapshot)
}

func decodeJSON(data []byte) (domain.GraphSnapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var document jsonDocument
	if err := decoder.Decode(&document); err != nil {
		return domain.GraphSnapshot{}, fmt.Errorf("decode graph JSON: %w", err)
	}
	if err := ensureEndOfJSON(decoder); err != nil {
		return domain.GraphSnapshot{}, err
	}
	if document.Version != JSONVersion {
		return domain.GraphSnapshot{}, fmt.Errorf("unsupported graph JSON version %d", document.Version)
	}
	if document.People == nil {
		return domain.GraphSnapshot{}, errors.New("graph JSON is missing people")
	}
	if document.Relationships == nil {
		return domain.GraphSnapshot{}, errors.New("graph JSON is missing relationships")
	}

	snapshot := domain.GraphSnapshot{
		People:      make([]domain.Person, 0, len(document.People)),
		ParentChild: make([]domain.ParentChild, 0, len(document.Relationships)),
	}
	for _, person := range document.People {
		birthDate, err := parseJSONDate(person.BirthDate)
		if err != nil {
			return domain.GraphSnapshot{}, fmt.Errorf("person %q birthDate: %w", person.ID, err)
		}
		deathDate, err := parseJSONDate(person.DeathDate)
		if err != nil {
			return domain.GraphSnapshot{}, fmt.Errorf("person %q deathDate: %w", person.ID, err)
		}
		snapshot.People = append(snapshot.People, domain.Person{
			ID:        person.ID,
			Name:      person.Name,
			BirthDate: birthDate,
			DeathDate: deathDate,
			CreatedAt: person.CreatedAt,
			UpdatedAt: person.UpdatedAt,
		})
	}
	for _, relation := range document.Relationships {
		snapshot.ParentChild = append(snapshot.ParentChild, domain.ParentChild{
			ParentID: relation.ParentID,
			ChildID:  relation.ChildID,
		})
	}
	return snapshot, nil
}

func ensureEndOfJSON(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return errors.New("graph JSON contains more than one document")
	default:
		return fmt.Errorf("decode graph JSON after document: %w", err)
	}
}

func jsonDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}

func parseJSONDate(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", *value, err)
	}
	return &parsed, nil
}
