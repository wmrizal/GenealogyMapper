package genealogy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"genealogymapper/internal/domain"
	"genealogymapper/internal/graph"
)

func TestServiceCreatePersonTrimsName(t *testing.T) {
	repository := newFakeRepository()
	service := NewService(repository)
	service.now = func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
	service.newID = func() string { return "person-1" }

	person, err := service.CreatePerson(context.Background(), CreatePersonInput{Name: "  Ada Lovelace  "})
	if err != nil {
		t.Fatalf("CreatePerson() error = %v", err)
	}
	if person.ID != "person-1" || person.Name != "Ada Lovelace" {
		t.Fatalf("CreatePerson() = %#v", person)
	}
	if !person.CreatedAt.Equal(person.UpdatedAt) {
		t.Fatalf("expected matching timestamps, got %v and %v", person.CreatedAt, person.UpdatedAt)
	}
}

func TestServiceRejectsInvalidPerson(t *testing.T) {
	service := NewService(newFakeRepository())

	_, err := service.CreatePerson(context.Background(), CreatePersonInput{Name: "  "})
	if !errors.Is(err, domain.ErrNameRequired) {
		t.Fatalf("CreatePerson() error = %v, want %v", err, domain.ErrNameRequired)
	}
}

func TestServiceUpdatePersonPreservesIdentityAndCreationTime(t *testing.T) {
	createdAt := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC)
	repository := newFakeRepository()
	repository.people["person-1"] = domain.Person{
		ID:        "person-1",
		Name:      "Old Name",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	service := NewService(repository)
	service.now = func() time.Time { return updatedAt }

	person, err := service.UpdatePerson(context.Background(), "person-1", CreatePersonInput{Name: "  New Name  "})
	if err != nil {
		t.Fatalf("UpdatePerson() error = %v", err)
	}
	if person.ID != "person-1" || person.Name != "New Name" {
		t.Fatalf("UpdatePerson() = %#v", person)
	}
	if !person.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %v, want %v", person.CreatedAt, createdAt)
	}
	if !person.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", person.UpdatedAt, updatedAt)
	}
}

func TestServiceAddParentAndChildUseCanonicalDirection(t *testing.T) {
	tests := []struct {
		name string
		call func(*Service) error
	}{
		{
			name: "parent",
			call: func(service *Service) error { return service.AddParent(context.Background(), "child-1", "parent-1") },
		},
		{
			name: "child",
			call: func(service *Service) error { return service.AddChild(context.Background(), "parent-1", "child-1") },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newFakeRepository()
			repository.seedPerson("parent-1")
			repository.seedPerson("child-1")

			if err := test.call(NewService(repository)); err != nil {
				t.Fatalf("relation command error = %v", err)
			}
			want := domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"}
			if !repository.relations[want] {
				t.Fatalf("relations = %#v, want %v", repository.relations, want)
			}
		})
	}
}

func TestServiceRejectsSelfAndDuplicateRelations(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("person-1")
	service := NewService(repository)

	if err := service.AddParent(context.Background(), "person-1", "person-1"); !errors.Is(err, domain.ErrSelfRelationship) {
		t.Fatalf("self relation error = %v, want %v", err, domain.ErrSelfRelationship)
	}

	repository.seedPerson("person-2")
	if err := service.AddChild(context.Background(), "person-1", "person-2"); err != nil {
		t.Fatalf("first relation error = %v", err)
	}
	if err := service.AddChild(context.Background(), "person-1", "person-2"); !errors.Is(err, graph.ErrRelationExists) {
		t.Fatalf("duplicate relation error = %v, want %v", err, graph.ErrRelationExists)
	}
}

func TestServiceRejectsMissingRelationEndpoint(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("parent-1")

	err := NewService(repository).AddChild(context.Background(), "parent-1", "missing-child")
	if !errors.Is(err, graph.ErrPersonNotFound) {
		t.Fatalf("missing endpoint error = %v, want %v", err, graph.ErrPersonNotFound)
	}
}

func TestServiceCreatePersonWithChildCreatesParentForSelectedPerson(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("child-1")
	service := NewService(repository)
	service.newID = func() string { return "parent-1" }

	person, err := service.CreatePersonWithChild(context.Background(), CreatePersonInput{Name: "Parent"}, "child-1")
	if err != nil {
		t.Fatalf("CreatePersonWithChild() error = %v", err)
	}
	if person.ID != "parent-1" {
		t.Fatalf("created person ID = %q, want %q", person.ID, "parent-1")
	}
	if _, exists := repository.people["parent-1"]; !exists {
		t.Fatal("new parent was not persisted")
	}
	want := domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"}
	if !repository.relations[want] {
		t.Fatalf("relations = %#v, want %v", repository.relations, want)
	}
}

func TestServiceCreatePersonWithParentCreatesChildForSelectedPerson(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("parent-1")
	service := NewService(repository)
	service.newID = func() string { return "child-1" }

	person, err := service.CreatePersonWithParent(context.Background(), CreatePersonInput{Name: "Child"}, "parent-1")
	if err != nil {
		t.Fatalf("CreatePersonWithParent() error = %v", err)
	}
	if person.ID != "child-1" {
		t.Fatalf("created person ID = %q, want %q", person.ID, "child-1")
	}
	if _, exists := repository.people["child-1"]; !exists {
		t.Fatal("new child was not persisted")
	}
	want := domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"}
	if !repository.relations[want] {
		t.Fatalf("relations = %#v, want %v", repository.relations, want)
	}
}

func TestServiceCreatePersonWithChildIsAtomic(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("child-1")
	service := NewService(repository)
	service.newID = func() string { return "parent-1" }

	person, err := service.CreatePersonWithChild(context.Background(), CreatePersonInput{Name: "Parent"}, "child-1")
	if err != nil {
		t.Fatalf("CreatePersonWithChild() error = %v", err)
	}
	if person.ID != "parent-1" || !repository.relations[domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"}] {
		t.Fatalf("atomic create did not persist person and relation: person=%#v relations=%#v", person, repository.relations)
	}

	repository = newFakeRepository()
	repository.seedPerson("child-1")
	repository.seedRelation(domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"})
	service = NewService(repository)
	service.newID = func() string { return "parent-1" }

	_, err = service.CreatePersonWithChild(context.Background(), CreatePersonInput{Name: "Parent"}, "child-1")
	if !errors.Is(err, graph.ErrRelationExists) {
		t.Fatalf("duplicate atomic create error = %v, want %v", err, graph.ErrRelationExists)
	}
	if _, exists := repository.people["parent-1"]; exists {
		t.Fatal("duplicate atomic create left the new person behind")
	}
}

func TestServiceNeighborhoodCountsSpouseAsOneHop(t *testing.T) {
	repository := newFakeRepository()
	for _, id := range []string{"me", "spouse", "child", "spouse-parent", "far"} {
		repository.seedPerson(id)
	}
	repository.seedRelation(domain.ParentChild{ParentID: "me", ChildID: "child"})
	repository.seedRelation(domain.ParentChild{ParentID: "spouse", ChildID: "child"})
	repository.seedRelation(domain.ParentChild{ParentID: "spouse-parent", ChildID: "spouse"})
	repository.seedRelation(domain.ParentChild{ParentID: "far", ChildID: "spouse-parent"})

	snapshot, err := NewService(repository).GetNeighborhood(context.Background(), "me")
	if err != nil {
		t.Fatalf("GetNeighborhood() error = %v", err)
	}
	got := map[string]bool{}
	for _, person := range snapshot.People {
		got[person.ID] = true
	}
	if !got["spouse"] || !got["spouse-parent"] || got["far"] {
		t.Fatalf("neighborhood people = %v, want spouse and spouse-parent but not far", got)
	}
}

func TestServiceJSONRoundTrip(t *testing.T) {
	birthDate := time.Date(1985, time.April, 12, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 6, time.UTC)
	repository := newFakeRepository()
	repository.people["parent-1"] = domain.Person{
		ID: "parent-1", Name: "Parent", BirthDate: &birthDate, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	repository.people["child-1"] = domain.Person{
		ID: "child-1", Name: "Child", CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	repository.seedRelation(domain.ParentChild{ParentID: "parent-1", ChildID: "child-1"})
	service := NewService(repository)

	data, err := service.ExportJSON(context.Background())
	if err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}
	if !strings.Contains(string(data), `"version": 1`) {
		t.Fatalf("ExportJSON() = %s", data)
	}

	imported := newFakeRepository()
	if err := NewService(imported).ImportJSON(context.Background(), data); err != nil {
		t.Fatalf("ImportJSON() error = %v", err)
	}
	if len(imported.people) != 2 || len(imported.relations) != 1 {
		t.Fatalf("imported graph = %#v, %#v", imported.people, imported.relations)
	}
	if got := imported.people["parent-1"].BirthDate; got == nil || !got.Equal(birthDate) {
		t.Fatalf("imported birth date = %v, want %v", got, birthDate)
	}
	if !imported.people["parent-1"].CreatedAt.Equal(createdAt) {
		t.Fatalf("imported CreatedAt = %v, want %v", imported.people["parent-1"].CreatedAt, createdAt)
	}
}

func TestServiceImportJSONRejectsInvalidGraphWithoutReplacingIt(t *testing.T) {
	repository := newFakeRepository()
	repository.seedPerson("existing")
	service := NewService(repository)

	err := service.ImportJSON(context.Background(), []byte(`{
		"version": 1,
		"people": [{"id": "parent", "name": "Parent", "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-01T00:00:00Z"}],
		"relationships": [{"parentId": "parent", "childId": "missing"}]
	}`))
	if err == nil {
		t.Fatal("ImportJSON() error = nil, want missing relationship endpoint")
	}
	if len(repository.people) != 1 {
		t.Fatalf("repository people = %#v, want existing graph unchanged", repository.people)
	}
}

type fakeRepository struct {
	people           map[string]domain.Person
	relations        map[domain.ParentChild]bool
	neighborhoodHops int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		people:    make(map[string]domain.Person),
		relations: make(map[domain.ParentChild]bool),
	}
}

func (r *fakeRepository) seedPerson(id string) {
	r.people[id] = domain.Person{ID: id, Name: id}
}

func (r *fakeRepository) seedRelation(relation domain.ParentChild) {
	r.relations[relation] = true
}

func (r *fakeRepository) CreatePerson(_ context.Context, person domain.Person) (domain.Person, error) {
	if _, exists := r.people[person.ID]; exists {
		return domain.Person{}, graph.ErrPersonExists
	}
	r.people[person.ID] = person
	return person, nil
}

func (r *fakeRepository) UpdatePerson(_ context.Context, person domain.Person) (domain.Person, error) {
	if _, exists := r.people[person.ID]; !exists {
		return domain.Person{}, graph.ErrPersonNotFound
	}
	r.people[person.ID] = person
	return person, nil
}

func (r *fakeRepository) CreatePersonAndLink(_ context.Context, person domain.Person, relation domain.ParentChild) (domain.Person, error) {
	if _, exists := r.people[person.ID]; exists {
		return domain.Person{}, graph.ErrPersonExists
	}
	if !r.endpointExists(person, relation.ParentID) || !r.endpointExists(person, relation.ChildID) {
		return domain.Person{}, graph.ErrPersonNotFound
	}
	if r.relations[relation] {
		return domain.Person{}, graph.ErrRelationExists
	}
	r.people[person.ID] = person
	r.relations[relation] = true
	return person, nil
}

func (r *fakeRepository) endpointExists(person domain.Person, id string) bool {
	return id == person.ID || func() bool {
		_, exists := r.people[id]
		return exists
	}()
}

func (r *fakeRepository) GetPerson(_ context.Context, personID string) (domain.Person, error) {
	person, exists := r.people[personID]
	if !exists {
		return domain.Person{}, graph.ErrPersonNotFound
	}
	return person, nil
}

func (r *fakeRepository) SearchPeople(_ context.Context, _ string) ([]domain.Person, error) {
	people := make([]domain.Person, 0, len(r.people))
	for _, person := range r.people {
		people = append(people, person)
	}
	return people, nil
}

func (r *fakeRepository) AddParentChild(_ context.Context, relation domain.ParentChild) (bool, error) {
	if _, exists := r.people[relation.ParentID]; !exists {
		return false, graph.ErrPersonNotFound
	}
	if _, exists := r.people[relation.ChildID]; !exists {
		return false, graph.ErrPersonNotFound
	}
	if r.relations[relation] {
		return false, nil
	}
	r.relations[relation] = true
	return true, nil
}

func (r *fakeRepository) GetSnapshot(_ context.Context) (domain.GraphSnapshot, error) {
	people := make([]domain.Person, 0, len(r.people))
	for _, person := range r.people {
		people = append(people, person)
	}
	relations := make([]domain.ParentChild, 0, len(r.relations))
	for relation := range r.relations {
		relations = append(relations, relation)
	}
	return domain.GraphSnapshot{People: people, ParentChild: relations}, nil
}

func (r *fakeRepository) ReplaceSnapshot(_ context.Context, snapshot domain.GraphSnapshot) error {
	r.people = make(map[string]domain.Person, len(snapshot.People))
	for _, person := range snapshot.People {
		r.people[person.ID] = person
	}
	r.relations = make(map[domain.ParentChild]bool, len(snapshot.ParentChild))
	for _, relation := range snapshot.ParentChild {
		r.relations[relation] = true
	}
	return nil
}

func (r *fakeRepository) GetNeighborhood(_ context.Context, personID string, maxHops int) (domain.GraphSnapshot, error) {
	if _, exists := r.people[personID]; !exists {
		return domain.GraphSnapshot{}, graph.ErrPersonNotFound
	}
	r.neighborhoodHops = maxHops
	return domain.GraphSnapshot{People: []domain.Person{r.people[personID]}}, nil
}
