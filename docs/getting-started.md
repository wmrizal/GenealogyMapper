# Getting started

Genealogy Mapper is a Windows desktop application written in Go. It stores people and parent-child relationships in Neo4j.

## Prerequisites

- Windows
- Go 1.23 or newer
- Docker Desktop with Compose

## Start Neo4j

From the project root, copy `.env.example` to `.env` if you want to customize the local credentials. The application automatically reads `.env` from its working directory. Set `GENEALOGY_CONFIG_FILE` to use another file. Missing files and missing settings fall back to the local development defaults; process environment variables override file values. These defaults are not production credentials.

Start the database:

```text
docker compose up -d
```

Check that the container is healthy:

```text
docker compose ps
```

Neo4j exposes Bolt on `localhost:7687`. Its local diagnostic browser is available at [http://localhost:7474](http://localhost:7474).

## Configure and run the desktop app

The application reads `GENEALOGY_NEO4J_URI`, `GENEALOGY_NEO4J_USERNAME`, `GENEALOGY_NEO4J_PASSWORD`, and `GENEALOGY_NEO4J_DATABASE` from `.env` or the configured file. In PowerShell, environment variables can override the file:

```powershell
$env:GENEALOGY_NEO4J_URI = "bolt://localhost:7687"
$env:GENEALOGY_NEO4J_USERNAME = "neo4j"
$env:GENEALOGY_NEO4J_PASSWORD = "genealogy-dev-password"
$env:GENEALOGY_NEO4J_DATABASE = "neo4j"
go run .
```

On first connection, the application creates a unique constraint for `Person.id`. The desktop window then opens with a people search pane, graph canvas, and selected-person inspector.

## Use the mapper

1. Choose **New Person** and enter a required name. Birth and death dates are optional.
2. Select a person in the people pane or graph.
3. Choose **Edit Person** to update the selected person's name or dates while keeping its stable ID.
4. Choose **Add Parent** or **Add Child**.
5. Link an existing person or choose **Create New** to create and link the person in one transaction.
6. Use middle- or right-drag to pan, the mouse wheel to zoom, and **Fit Graph** to recenter the current neighborhood.
7. Choose **Export JSON** to save the complete genealogy, including person IDs, dates, timestamps, and relationships.
8. Choose **Import JSON** to replace the current genealogy from a previously exported file. The import is validated before it changes the database.

Relationships are stored in one direction only: `(:Person)-[:PARENT_OF]->(:Person)`. The UI's parent and child commands normalize to this representation, so repeated links do not create duplicate edges.

JSON files use a versioned document with `version`, `people`, and `relationships` fields. Person dates use `YYYY-MM-DD`; timestamps use RFC 3339.

## Build and test

```text
go test ./...
go vet ./...
go build ./...
```

Stop the local database with:

```text
docker compose down
```

Use `docker compose down -v` only when you intentionally want to delete the persisted Neo4j data volume.