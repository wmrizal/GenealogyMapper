package graph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"genealogymapper/internal/domain"
)

func TestNeo4jRepositoryIntegration(t *testing.T) {
	if os.Getenv("NEO4J_INTEGRATION") != "1" {
		t.Skip("set NEO4J_INTEGRATION=1 to run against Neo4j")
	}

	config := Neo4jConfig{
		URI:      valueOrFallback("GENEALOGY_NEO4J_URI", "NEO4J_URI", "bolt://localhost:7687"),
		Username: valueOrFallback("GENEALOGY_NEO4J_USERNAME", "NEO4J_USERNAME", "neo4j"),
		Password: valueOrFallback("GENEALOGY_NEO4J_PASSWORD", "NEO4J_PASSWORD", ""),
		Database: valueOrFallback("GENEALOGY_NEO4J_DATABASE", "NEO4J_DATABASE", "neo4j"),
	}
	if config.Password == "" {
		t.Fatal("set GENEALOGY_NEO4J_PASSWORD or NEO4J_PASSWORD")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := NewNeo4jRepository(ctx, config)
	if err != nil {
		t.Fatalf("NewNeo4jRepository() error = %v", err)
	}
	defer repository.Close(ctx)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	parentID := "integration-parent-" + suffix
	childID := "integration-child-" + suffix
	defer deleteIntegrationPeople(t, ctx, repository, parentID, childID)

	parent := domain.Person{ID: parentID, Name: "Integration Parent", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	child := domain.Person{ID: childID, Name: "Integration Child", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := repository.CreatePerson(ctx, parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	if _, err := repository.CreatePerson(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}
	if _, err := repository.CreatePerson(ctx, parent); !errors.Is(err, ErrPersonExists) {
		t.Fatalf("duplicate person error = %v, want %v", err, ErrPersonExists)
	}
	updatedParent := parent
	updatedParent.Name = "Updated Parent"
	updatedParent.UpdatedAt = time.Now().UTC()
	updated, err := repository.UpdatePerson(ctx, updatedParent)
	if err != nil {
		t.Fatalf("UpdatePerson() error = %v", err)
	}
	if updated.ID != parentID || updated.Name != "Updated Parent" {
		t.Fatalf("updated parent = %#v", updated)
	}
	if !updated.CreatedAt.Equal(parent.CreatedAt) {
		t.Fatalf("updated CreatedAt = %v, want %v", updated.CreatedAt, parent.CreatedAt)
	}

	relation := domain.ParentChild{ParentID: parentID, ChildID: childID}
	created, err := repository.AddParentChild(ctx, relation)
	if err != nil || !created {
		t.Fatalf("first relation = %v, %v; want true, nil", created, err)
	}
	created, err = repository.AddParentChild(ctx, relation)
	if err != nil || created {
		t.Fatalf("duplicate relation = %v, %v; want false, nil", created, err)
	}

	snapshot, err := repository.GetNeighborhood(ctx, childID, 1)
	if err != nil {
		t.Fatalf("GetNeighborhood() error = %v", err)
	}
	if len(snapshot.People) != 2 || len(snapshot.ParentChild) != 1 {
		t.Fatalf("snapshot = %#v, want two people and one relation", snapshot)
	}
	if snapshot.ParentChild[0] != relation {
		t.Fatalf("snapshot relation = %#v, want %#v", snapshot.ParentChild[0], relation)
	}
}

func TestNeo4jRepositoryCreatePersonAndLinkWithNewParent(t *testing.T) {
	if os.Getenv("NEO4J_INTEGRATION") != "1" {
		t.Skip("set NEO4J_INTEGRATION=1 to run against Neo4j")
	}

	config := Neo4jConfig{
		URI:      valueOrFallback("GENEALOGY_NEO4J_URI", "NEO4J_URI", "bolt://localhost:7687"),
		Username: valueOrFallback("GENEALOGY_NEO4J_USERNAME", "NEO4J_USERNAME", "neo4j"),
		Password: valueOrFallback("GENEALOGY_NEO4J_PASSWORD", "NEO4J_PASSWORD", ""),
		Database: valueOrFallback("GENEALOGY_NEO4J_DATABASE", "NEO4J_DATABASE", "neo4j"),
	}
	if config.Password == "" {
		t.Fatal("set GENEALOGY_NEO4J_PASSWORD or NEO4J_PASSWORD")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := NewNeo4jRepository(ctx, config)
	if err != nil {
		t.Fatalf("NewNeo4jRepository() error = %v", err)
	}
	defer repository.Close(ctx)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	parentID := "atomic-parent-" + suffix
	childID := "atomic-child-" + suffix
	defer deleteIntegrationPeople(t, ctx, repository, parentID, childID)

	child := domain.Person{ID: childID, Name: "Atomic Child", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := repository.CreatePerson(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	parent := domain.Person{ID: parentID, Name: "Atomic Parent", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	relation := domain.ParentChild{ParentID: parentID, ChildID: childID}
	created, err := repository.CreatePersonAndLink(ctx, parent, relation)
	if err != nil {
		t.Fatalf("CreatePersonAndLink() error = %v", err)
	}
	if created.ID != parentID {
		t.Fatalf("created parent ID = %q, want %q", created.ID, parentID)
	}

	snapshot, err := repository.GetNeighborhood(ctx, parentID, 1)
	if err != nil {
		t.Fatalf("GetNeighborhood() error = %v", err)
	}
	if len(snapshot.People) != 2 || len(snapshot.ParentChild) != 1 {
		t.Fatalf("snapshot = %#v, want two people and one relation", snapshot)
	}
	if snapshot.ParentChild[0] != relation {
		t.Fatalf("snapshot relation = %#v, want %#v", snapshot.ParentChild[0], relation)
	}
}

func TestNeo4jRepositoryCreatePersonAndLinkWithNewChild(t *testing.T) {
	if os.Getenv("NEO4J_INTEGRATION") != "1" {
		t.Skip("set NEO4J_INTEGRATION=1 to run against Neo4j")
	}

	config := Neo4jConfig{
		URI:      valueOrFallback("GENEALOGY_NEO4J_URI", "NEO4J_URI", "bolt://localhost:7687"),
		Username: valueOrFallback("GENEALOGY_NEO4J_USERNAME", "NEO4J_USERNAME", "neo4j"),
		Password: valueOrFallback("GENEALOGY_NEO4J_PASSWORD", "NEO4J_PASSWORD", ""),
		Database: valueOrFallback("GENEALOGY_NEO4J_DATABASE", "NEO4J_DATABASE", "neo4j"),
	}
	if config.Password == "" {
		t.Fatal("set GENEALOGY_NEO4J_PASSWORD or NEO4J_PASSWORD")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository, err := NewNeo4jRepository(ctx, config)
	if err != nil {
		t.Fatalf("NewNeo4jRepository() error = %v", err)
	}
	defer repository.Close(ctx)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	parentID := "atomic-existing-parent-" + suffix
	childID := "atomic-new-child-" + suffix
	defer deleteIntegrationPeople(t, ctx, repository, parentID, childID)

	parent := domain.Person{ID: parentID, Name: "Atomic Existing Parent", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if _, err := repository.CreatePerson(ctx, parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child := domain.Person{ID: childID, Name: "Atomic New Child", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	relation := domain.ParentChild{ParentID: parentID, ChildID: childID}
	created, err := repository.CreatePersonAndLink(ctx, child, relation)
	if err != nil {
		t.Fatalf("CreatePersonAndLink() error = %v", err)
	}
	if created.ID != childID {
		t.Fatalf("created child ID = %q, want %q", created.ID, childID)
	}

	snapshot, err := repository.GetNeighborhood(ctx, childID, 1)
	if err != nil {
		t.Fatalf("GetNeighborhood() error = %v", err)
	}
	if len(snapshot.People) != 2 || len(snapshot.ParentChild) != 1 {
		t.Fatalf("snapshot = %#v, want two people and one relation", snapshot)
	}
	if snapshot.ParentChild[0] != relation {
		t.Fatalf("snapshot relation = %#v, want %#v", snapshot.ParentChild[0], relation)
	}
}

func deleteIntegrationPeople(t *testing.T, ctx context.Context, repository *Neo4jRepository, ids ...string) {
	t.Helper()
	session := repository.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: repository.database})
	defer session.Close(ctx)
	result, err := session.Run(ctx, `MATCH (person:Person) WHERE person.id IN $ids DETACH DELETE person`, map[string]any{"ids": ids})
	if err != nil {
		t.Errorf("cleanup query: %v", err)
		return
	}
	if _, err := result.Consume(ctx); err != nil {
		t.Errorf("cleanup consume: %v", err)
	}
}

func valueOrFallback(primary, secondary, fallback string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	if value := os.Getenv(secondary); value != "" {
		return value
	}
	return fallback
}
