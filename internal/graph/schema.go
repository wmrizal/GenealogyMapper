package graph

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

const personIDConstraintCypher = `
CREATE CONSTRAINT person_id_unique IF NOT EXISTS
FOR (person:Person)
REQUIRE person.id IS UNIQUE`

func ensureSchema(ctx context.Context, session neo4j.SessionWithContext) error {
	result, err := session.Run(ctx, personIDConstraintCypher, nil)
	if err != nil {
		return err
	}
	_, err = result.Consume(ctx)
	return err
}
