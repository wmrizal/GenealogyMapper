package graph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"genealogymapper/internal/domain"
)

const (
	defaultDatabase  = "neo4j"
	maxSearchResults = 100
	dateFormat       = "2006-01-02"
	timestampFormat  = time.RFC3339Nano
)

type Neo4jConfig struct {
	URI      string
	Username string
	Password string
	Database string
}

type Neo4jRepository struct {
	driver   neo4j.DriverWithContext
	database string
}

func NewNeo4jRepository(ctx context.Context, config Neo4jConfig) (*Neo4jRepository, error) {
	if strings.TrimSpace(config.URI) == "" {
		return nil, errors.New("neo4j URI is required")
	}
	if strings.TrimSpace(config.Username) == "" {
		return nil, errors.New("neo4j username is required")
	}
	if config.Database == "" {
		config.Database = defaultDatabase
	}

	driver, err := neo4j.NewDriverWithContext(config.URI, neo4j.BasicAuth(config.Username, config.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("create neo4j driver: %w", err)
	}
	repository := &Neo4jRepository{driver: driver, database: config.Database}
	if err := driver.VerifyConnectivity(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("verify neo4j connectivity: %w", err)
	}

	session := driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: config.Database})
	if err := ensureSchema(ctx, session); err != nil {
		_ = session.Close(ctx)
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("initialize neo4j schema: %w", err)
	}
	if err := session.Close(ctx); err != nil {
		_ = driver.Close(ctx)
		return nil, fmt.Errorf("close neo4j schema session: %w", err)
	}
	return repository, nil
}

func (r *Neo4jRepository) Close(ctx context.Context) error {
	return r.driver.Close(ctx)
}

func (r *Neo4jRepository) CreatePerson(ctx context.Context, person domain.Person) (domain.Person, error) {
	if err := person.Validate(); err != nil {
		return domain.Person{}, err
	}
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, `
CREATE (person:Person {
		id: $id,
		name: $name,
		birthDate: $birthDate,
		deathDate: $deathDate,
		createdAt: $createdAt,
		updatedAt: $updatedAt
})
RETURN person`, personParameters(person))
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			record, err := result.Single(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			return personFromValue(recordValue(record, "person"))
		})
	})
	if err != nil {
		return domain.Person{}, err
	}
	created, ok := value.(domain.Person)
	if !ok {
		return domain.Person{}, errors.New("neo4j returned an invalid person")
	}
	return created, nil
}

func (r *Neo4jRepository) UpdatePerson(ctx context.Context, person domain.Person) (domain.Person, error) {
	if err := person.Validate(); err != nil {
		return domain.Person{}, err
	}
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, `
MATCH (person:Person {id: $id})
SET person.name = $name,
		person.birthDate = $birthDate,
		person.deathDate = $deathDate,
		person.updatedAt = $updatedAt
RETURN person`, personParameters(person))
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			records, err := result.Collect(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if len(records) == 0 {
				return nil, ErrPersonNotFound
			}
			return personFromValue(recordValue(records[0], "person"))
		})
	})
	if err != nil {
		return domain.Person{}, err
	}
	updated, ok := value.(domain.Person)
	if !ok {
		return domain.Person{}, errors.New("neo4j returned an invalid person")
	}
	return updated, nil
}

func (r *Neo4jRepository) CreatePersonAndLink(ctx context.Context, person domain.Person, relation domain.ParentChild) (domain.Person, error) {
	if err := person.Validate(); err != nil {
		return domain.Person{}, err
	}
	if err := relation.Validate(); err != nil {
		return domain.Person{}, err
	}

	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			checkResult, err := tx.Run(ctx, `
OPTIONAL MATCH (existing:Person {id: $personID})
OPTIONAL MATCH (parent:Person {id: $parentID})
OPTIONAL MATCH (child:Person {id: $childID})
OPTIONAL MATCH (parent)-[existingRelation:PARENT_OF]->(child)
RETURN existing, parent, child, existingRelation`, map[string]any{
				"personID": relationPersonID(person, relation),
				"parentID": relation.ParentID,
				"childID":  relation.ChildID,
			})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			record, err := checkResult.Single(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if value, exists := record.Get("existing"); exists && value != nil {
				return nil, ErrPersonExists
			}
			parent, parentExists := record.Get("parent")
			child, childExists := record.Get("child")
			if (relation.ParentID != person.ID && (!parentExists || parent == nil)) ||
				(relation.ChildID != person.ID && (!childExists || child == nil)) {
				return nil, ErrPersonNotFound
			}
			if value, exists := record.Get("existingRelation"); exists && value != nil {
				return nil, ErrRelationExists
			}

			createResult, err := tx.Run(ctx, `
CREATE (person:Person {
		id: $id,
		name: $name,
		birthDate: $birthDate,
		deathDate: $deathDate,
		createdAt: $createdAt,
		updatedAt: $updatedAt
})
WITH person
MATCH (parent:Person {id: $parentID}), (child:Person {id: $childID})
CREATE (parent)-[:PARENT_OF]->(child)
RETURN person`, personParametersWithRelation(person, relation))
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			createdRecord, err := createResult.Single(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			return personFromValue(recordValue(createdRecord, "person"))
		})
	})
	if err != nil {
		return domain.Person{}, err
	}
	created, ok := value.(domain.Person)
	if !ok {
		return domain.Person{}, errors.New("neo4j returned an invalid person")
	}
	return created, nil
}

func (r *Neo4jRepository) GetPerson(ctx context.Context, personID string) (domain.Person, error) {
	if strings.TrimSpace(personID) == "" {
		return domain.Person{}, ErrPersonNotFound
	}
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, `MATCH (person:Person {id: $id}) RETURN person`, map[string]any{"id": personID})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			records, err := result.Collect(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if len(records) == 0 {
				return nil, ErrPersonNotFound
			}
			return personFromValue(recordValue(records[0], "person"))
		})
	})
	if err != nil {
		return domain.Person{}, err
	}
	person, ok := value.(domain.Person)
	if !ok {
		return domain.Person{}, errors.New("neo4j returned an invalid person")
	}
	return person, nil
}

func (r *Neo4jRepository) SearchPeople(ctx context.Context, query string) ([]domain.Person, error) {
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, `
MATCH (person:Person)
WHERE $query = '' OR toLower(person.name) CONTAINS toLower($query)
RETURN person
ORDER BY toLower(person.name), person.id
LIMIT $limit`, map[string]any{
				"query": strings.TrimSpace(query),
				"limit": maxSearchResults,
			})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			records, err := result.Collect(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			people := make([]domain.Person, 0, len(records))
			for _, record := range records {
				person, err := personFromValue(recordValue(record, "person"))
				if err != nil {
					return nil, err
				}
				people = append(people, person)
			}
			return people, nil
		})
	})
	if err != nil {
		return nil, err
	}
	people, ok := value.([]domain.Person)
	if !ok {
		return nil, errors.New("neo4j returned invalid people")
	}
	return people, nil
}

func (r *Neo4jRepository) GetSnapshot(ctx context.Context) (domain.GraphSnapshot, error) {
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, `
OPTIONAL MATCH (person:Person)
WITH collect(person) AS people
OPTIONAL MATCH (parent:Person)-[:PARENT_OF]->(child:Person)
WITH people, collect(CASE
	WHEN parent IS NULL THEN NULL
	ELSE {parentID: parent.id, childID: child.id}
END) AS relationCandidates
RETURN people, [relation IN relationCandidates WHERE relation IS NOT NULL] AS relations`, nil)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			record, err := result.Single(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			return snapshotFromRecord(record)
		})
	})
	if err != nil {
		return domain.GraphSnapshot{}, err
	}
	snapshot, ok := value.(domain.GraphSnapshot)
	if !ok {
		return domain.GraphSnapshot{}, errors.New("neo4j returned an invalid graph snapshot")
	}
	return snapshot, nil
}

func (r *Neo4jRepository) ReplaceSnapshot(ctx context.Context, snapshot domain.GraphSnapshot) error {
	if err := ValidateSnapshot(snapshot); err != nil {
		return err
	}

	people := make([]map[string]any, 0, len(snapshot.People))
	for _, person := range snapshot.People {
		people = append(people, personParameters(person))
	}
	relations := make([]map[string]any, 0, len(snapshot.ParentChild))
	for _, relation := range snapshot.ParentChild {
		relations = append(relations, map[string]any{
			"parentID": relation.ParentID,
			"childID":  relation.ChildID,
		})
	}

	_, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			clearResult, err := tx.Run(ctx, `MATCH (person:Person) DETACH DELETE person`, nil)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if _, err := clearResult.Consume(ctx); err != nil {
				return nil, translateNeo4jError(err)
			}

			if len(people) > 0 {
				createPeopleResult, err := tx.Run(ctx, `
UNWIND $people AS data
CREATE (:Person {
	id: data.id,
	name: data.name,
	birthDate: data.birthDate,
	deathDate: data.deathDate,
	createdAt: data.createdAt,
	updatedAt: data.updatedAt
})`, map[string]any{"people": people})
				if err != nil {
					return nil, translateNeo4jError(err)
				}
				if _, err := createPeopleResult.Consume(ctx); err != nil {
					return nil, translateNeo4jError(err)
				}
			}

			if len(relations) > 0 {
				createRelationsResult, err := tx.Run(ctx, `
UNWIND $relationships AS relation
MATCH (parent:Person {id: relation.parentID}), (child:Person {id: relation.childID})
CREATE (parent)-[:PARENT_OF]->(child)`, map[string]any{"relationships": relations})
				if err != nil {
					return nil, translateNeo4jError(err)
				}
				if _, err := createRelationsResult.Consume(ctx); err != nil {
					return nil, translateNeo4jError(err)
				}
			}
			return nil, nil
		})
	})
	return err
}

func (r *Neo4jRepository) AddParentChild(ctx context.Context, relation domain.ParentChild) (bool, error) {
	if err := relation.Validate(); err != nil {
		return false, err
	}
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			checkResult, err := tx.Run(ctx, `
OPTIONAL MATCH (parent:Person {id: $parentID})
OPTIONAL MATCH (child:Person {id: $childID})
OPTIONAL MATCH (parent)-[existing:PARENT_OF]->(child)
RETURN parent, child, existing`, map[string]any{
				"parentID": relation.ParentID,
				"childID":  relation.ChildID,
			})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			record, err := checkResult.Single(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			parent, parentExists := record.Get("parent")
			child, childExists := record.Get("child")
			if !parentExists || parent == nil || !childExists || child == nil {
				return nil, ErrPersonNotFound
			}
			if existing, exists := record.Get("existing"); exists && existing != nil {
				return false, nil
			}

			createResult, err := tx.Run(ctx, `
MATCH (parent:Person {id: $parentID}), (child:Person {id: $childID})
MERGE (parent)-[:PARENT_OF]->(child)
RETURN parent`, map[string]any{
				"parentID": relation.ParentID,
				"childID":  relation.ChildID,
			})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if _, err := createResult.Single(ctx); err != nil {
				return nil, translateNeo4jError(err)
			}
			return true, nil
		})
	})
	if err != nil {
		return false, err
	}
	created, ok := value.(bool)
	if !ok {
		return false, errors.New("neo4j returned an invalid relation result")
	}
	return created, nil
}

func (r *Neo4jRepository) GetNeighborhood(ctx context.Context, personID string, maxHops int) (domain.GraphSnapshot, error) {
	if strings.TrimSpace(personID) == "" {
		return domain.GraphSnapshot{}, ErrPersonNotFound
	}
	if maxHops < 0 {
		return domain.GraphSnapshot{}, errors.New("neighborhood hop limit cannot be negative")
	}
	value, err := r.withSession(ctx, func(session neo4j.SessionWithContext) (any, error) {
		return session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			query := fmt.Sprintf(`
MATCH path=(center:Person {id: $personID})-[:PARENT_OF*0..%d]-(person:Person)
WITH collect(DISTINCT person) AS people
UNWIND people AS parent
OPTIONAL MATCH (parent)-[:PARENT_OF]->(child:Person)
WHERE child IN people
WITH people, collect(DISTINCT CASE
	WHEN child IS NULL THEN NULL
	ELSE {parentID: parent.id, childID: child.id}
END) AS relationCandidates
RETURN people, [relation IN relationCandidates WHERE relation IS NOT NULL] AS relations`, maxHops)
			result, err := tx.Run(ctx, query, map[string]any{"personID": personID})
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			records, err := result.Collect(ctx)
			if err != nil {
				return nil, translateNeo4jError(err)
			}
			if len(records) == 0 {
				return nil, ErrPersonNotFound
			}
			return snapshotFromRecord(records[0])
		})
	})
	if err != nil {
		return domain.GraphSnapshot{}, err
	}
	snapshot, ok := value.(domain.GraphSnapshot)
	if !ok {
		return domain.GraphSnapshot{}, errors.New("neo4j returned an invalid graph snapshot")
	}
	return snapshot, nil
}

func (r *Neo4jRepository) withSession(ctx context.Context, operation func(neo4j.SessionWithContext) (any, error)) (any, error) {
	session := r.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: r.database})
	defer func() { _ = session.Close(ctx) }()
	return operation(session)
}

func personParameters(person domain.Person) map[string]any {
	return map[string]any{
		"id":        person.ID,
		"name":      person.Name,
		"birthDate": optionalDate(person.BirthDate),
		"deathDate": optionalDate(person.DeathDate),
		"createdAt": person.CreatedAt.UTC().Format(timestampFormat),
		"updatedAt": person.UpdatedAt.UTC().Format(timestampFormat),
	}
}

func personParametersWithRelation(person domain.Person, relation domain.ParentChild) map[string]any {
	parameters := personParameters(person)
	parameters["personID"] = person.ID
	parameters["parentID"] = relation.ParentID
	parameters["childID"] = relation.ChildID
	return parameters
}

func relationPersonID(person domain.Person, relation domain.ParentChild) string {
	if relation.ParentID == person.ID || relation.ChildID == person.ID {
		return person.ID
	}
	return person.ID
}

func optionalDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(dateFormat)
}

func recordValue(record *neo4j.Record, key string) any {
	value, _ := record.Get(key)
	return value
}

func personFromValue(value any) (domain.Person, error) {
	node, ok := value.(neo4j.Node)
	if !ok {
		if pointer, pointerOK := value.(*neo4j.Node); pointerOK && pointer != nil {
			node = *pointer
			ok = true
		}
	}
	if !ok {
		return domain.Person{}, fmt.Errorf("neo4j returned %T instead of a person node", value)
	}
	id, err := stringProperty(node.Props, "id")
	if err != nil {
		return domain.Person{}, err
	}
	name, err := stringProperty(node.Props, "name")
	if err != nil {
		return domain.Person{}, err
	}
	birthDate, err := dateProperty(node.Props, "birthDate")
	if err != nil {
		return domain.Person{}, err
	}
	deathDate, err := dateProperty(node.Props, "deathDate")
	if err != nil {
		return domain.Person{}, err
	}
	createdAt, err := timestampProperty(node.Props, "createdAt")
	if err != nil {
		return domain.Person{}, err
	}
	updatedAt, err := timestampProperty(node.Props, "updatedAt")
	if err != nil {
		return domain.Person{}, err
	}
	return domain.Person{
		ID:        id,
		Name:      name,
		BirthDate: birthDate,
		DeathDate: deathDate,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func snapshotFromRecord(record *neo4j.Record) (domain.GraphSnapshot, error) {
	peopleValues, ok := recordValue(record, "people").([]any)
	if !ok {
		return domain.GraphSnapshot{}, fmt.Errorf("neo4j returned %T instead of people", recordValue(record, "people"))
	}
	people := make([]domain.Person, 0, len(peopleValues))
	for _, value := range peopleValues {
		person, err := personFromValue(value)
		if err != nil {
			return domain.GraphSnapshot{}, err
		}
		people = append(people, person)
	}
	relationValues, ok := recordValue(record, "relations").([]any)
	if !ok {
		return domain.GraphSnapshot{}, fmt.Errorf("neo4j returned %T instead of relations", recordValue(record, "relations"))
	}
	relations := make([]domain.ParentChild, 0, len(relationValues))
	for _, value := range relationValues {
		properties, ok := value.(map[string]any)
		if !ok {
			return domain.GraphSnapshot{}, fmt.Errorf("neo4j returned %T instead of a relation map", value)
		}
		parentID, err := stringProperty(properties, "parentID")
		if err != nil {
			return domain.GraphSnapshot{}, err
		}
		childID, err := stringProperty(properties, "childID")
		if err != nil {
			return domain.GraphSnapshot{}, err
		}
		relations = append(relations, domain.ParentChild{ParentID: parentID, ChildID: childID})
	}
	sort.Slice(people, func(i, j int) bool {
		if people[i].Name == people[j].Name {
			return people[i].ID < people[j].ID
		}
		return people[i].Name < people[j].Name
	})
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].ParentID == relations[j].ParentID {
			return relations[i].ChildID < relations[j].ChildID
		}
		return relations[i].ParentID < relations[j].ParentID
	})
	return domain.GraphSnapshot{People: people, ParentChild: relations}, nil
}

func stringProperty(properties map[string]any, key string) (string, error) {
	value, exists := properties[key]
	if !exists {
		return "", fmt.Errorf("neo4j person is missing %q", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("neo4j property %q has type %T, want string", key, value)
	}
	return text, nil
}

func dateProperty(properties map[string]any, key string) (*time.Time, error) {
	value, exists := properties[key]
	if !exists || value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("neo4j property %q has type %T, want date string", key, value)
	}
	parsed, err := time.Parse(dateFormat, text)
	if err != nil {
		return nil, fmt.Errorf("parse neo4j property %q: %w", key, err)
	}
	return &parsed, nil
}

func timestampProperty(properties map[string]any, key string) (time.Time, error) {
	value, exists := properties[key]
	if !exists || value == nil {
		return time.Time{}, nil
	}
	text, ok := value.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("neo4j property %q has type %T, want timestamp string", key, value)
	}
	parsed, err := time.Parse(timestampFormat, text)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse neo4j property %q: %w", key, err)
	}
	return parsed, nil
}

func translateNeo4jError(err error) error {
	var neo4jError *neo4j.Neo4jError
	if errors.As(err, &neo4jError) && neo4jError.Code == "Neo.ClientError.Schema.ConstraintValidationFailed" {
		return ErrPersonExists
	}
	return err
}
