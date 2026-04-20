# Backend

Go server for the Tower Defense game.

## Prerequisites

- Go 1.26+
- PostgreSQL 15+ (or run via `docker-compose up` from the repo root)
- `staticcheck` installed: `go install honnef.co/go/tools/cmd/staticcheck@latest`

## Setup

```bash
cp .env.example .env
# Edit .env with your local credentials

make migrate-up
make run
```

## Available targets

| Target         | Description                                       |
| -------------- | ------------------------------------------------- |
| `make run`     | Run the HTTP/WS server                            |
| `make test`    | Run all tests with the race detector              |
| `make lint`    | Run `go vet` and `staticcheck`                    |
| `make fmt`     | Format all Go source with `gofmt -s`              |
| `make migrate-up`   | Apply all pending migrations                 |
| `make migrate-down` | Roll back the last applied migration         |
| `make build`   | Compile server and migrate binaries into `bin/`   |

## Module path

`github.com/johannesniedens/towerdefense`
