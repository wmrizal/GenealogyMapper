package genealogy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"genealogymapper/internal/domain"
	"genealogymapper/internal/graph"
)

const DefaultNeighborhoodHops = 2

type CreatePersonInput struct {
	Name      string
	BirthDate *time.Time
	DeathDate *time.Time
}

type Service struct {
	repository graph.Repository
	now        func() time.Time
	newID      func() string
}

func NewService(repository graph.Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
		newID:      newPersonID,
	}
}

func (s *Service) CreatePerson(ctx context.Context, input CreatePersonInput) (domain.Person, error) {
	person, err := s.newPerson(input)
	if err != nil {
		return domain.Person{}, err
	}
	return s.repository.CreatePerson(ctx, person)
}

func (s *Service) UpdatePerson(ctx context.Context, personID string, input CreatePersonInput) (domain.Person, error) {
	person, err := domain.NewPerson(personID, input.Name, input.BirthDate, input.DeathDate, time.Time{})
	if err != nil {
		return domain.Person{}, err
	}
	existing, err := s.repository.GetPerson(ctx, person.ID)
	if err != nil {
		return domain.Person{}, err
	}
	person.CreatedAt = existing.CreatedAt
	person.UpdatedAt = s.now().UTC()
	return s.repository.UpdatePerson(ctx, person)
}

func (s *Service) CreatePersonWithParent(ctx context.Context, input CreatePersonInput, parentID string) (domain.Person, error) {
	person, err := s.newPerson(input)
	if err != nil {
		return domain.Person{}, err
	}
	relation := domain.ParentChild{ParentID: parentID, ChildID: person.ID}
	if err := relation.Validate(); err != nil {
		return domain.Person{}, err
	}
	return s.repository.CreatePersonAndLink(ctx, person, relation)
}

func (s *Service) CreatePersonWithChild(ctx context.Context, input CreatePersonInput, childID string) (domain.Person, error) {
	person, err := s.newPerson(input)
	if err != nil {
		return domain.Person{}, err
	}
	relation := domain.ParentChild{ParentID: person.ID, ChildID: childID}
	if err := relation.Validate(); err != nil {
		return domain.Person{}, err
	}
	return s.repository.CreatePersonAndLink(ctx, person, relation)
}

func (s *Service) AddParent(ctx context.Context, childID, parentID string) error {
	return s.addRelation(ctx, domain.ParentChild{ParentID: parentID, ChildID: childID})
}

func (s *Service) AddChild(ctx context.Context, parentID, childID string) error {
	return s.addRelation(ctx, domain.ParentChild{ParentID: parentID, ChildID: childID})
}

func (s *Service) SearchPeople(ctx context.Context, query string) ([]domain.Person, error) {
	return s.repository.SearchPeople(ctx, strings.TrimSpace(query))
}

func (s *Service) GetPerson(ctx context.Context, personID string) (domain.Person, error) {
	if strings.TrimSpace(personID) == "" {
		return domain.Person{}, graph.ErrPersonNotFound
	}
	return s.repository.GetPerson(ctx, personID)
}

func (s *Service) GetNeighborhood(ctx context.Context, personID string) (domain.GraphSnapshot, error) {
	if strings.TrimSpace(personID) == "" {
		return domain.GraphSnapshot{}, graph.ErrPersonNotFound
	}
	if _, err := s.repository.GetPerson(ctx, personID); err != nil {
		return domain.GraphSnapshot{}, err
	}
	snapshot, err := s.repository.GetSnapshot(ctx)
	if err != nil {
		return domain.GraphSnapshot{}, err
	}
	return neighborhood(snapshot, personID, DefaultNeighborhoodHops), nil
}

// neighborhood keeps people within maxHops of personID; parent, child and co-parent (spouse) links each cost one hop.
func neighborhood(snapshot domain.GraphSnapshot, personID string, maxHops int) domain.GraphSnapshot {
	parents := make(map[string][]string)
	children := make(map[string][]string)
	for _, relation := range snapshot.ParentChild {
		parents[relation.ChildID] = append(parents[relation.ChildID], relation.ParentID)
		children[relation.ParentID] = append(children[relation.ParentID], relation.ChildID)
	}
	adjacent := func(id string) []string {
		var ids []string
		ids = append(ids, parents[id]...)
		ids = append(ids, children[id]...)
		for _, childID := range children[id] {
			for _, spouseID := range parents[childID] {
				if spouseID != id {
					ids = append(ids, spouseID)
				}
			}
		}
		return ids
	}

	visited := map[string]bool{personID: true}
	frontier := []string{personID}
	for hop := 0; hop < maxHops && len(frontier) > 0; hop++ {
		var next []string
		for _, id := range frontier {
			for _, neighbor := range adjacent(id) {
				if !visited[neighbor] {
					visited[neighbor] = true
					next = append(next, neighbor)
				}
			}
		}
		frontier = next
	}

	result := domain.GraphSnapshot{}
	for _, person := range snapshot.People {
		if visited[person.ID] {
			result.People = append(result.People, person)
		}
	}
	for _, relation := range snapshot.ParentChild {
		if visited[relation.ParentID] && visited[relation.ChildID] {
			result.ParentChild = append(result.ParentChild, relation)
		}
	}
	return result
}

func (s *Service) GetFullGraph(ctx context.Context) (domain.GraphSnapshot, error) {
	return s.repository.GetSnapshot(ctx)
}

func (s *Service) newPerson(input CreatePersonInput) (domain.Person, error) {
	return domain.NewPerson(s.newID(), input.Name, input.BirthDate, input.DeathDate, s.now().UTC())
}

func (s *Service) addRelation(ctx context.Context, relation domain.ParentChild) error {
	if err := relation.Validate(); err != nil {
		return err
	}
	created, err := s.repository.AddParentChild(ctx, relation)
	if err != nil {
		return err
	}
	if !created {
		return graph.ErrRelationExists
	}
	return nil
}

func newPersonID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "person-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return "person-" + hex.EncodeToString(raw[:])
}
