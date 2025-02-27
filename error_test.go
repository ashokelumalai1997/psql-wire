package wire

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jeroenrinzema/psql-wire/codes"
	psqlerr "github.com/jeroenrinzema/psql-wire/errors"
	"github.com/lib/pq/oid"
	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/assert"
)

func TestErrorCode(t *testing.T) {
	handler := func(ctx context.Context, query string) (PreparedStatementFn, []oid.Oid, Columns, error) {
		statement := func(ctx context.Context, writer DataWriter, parameters []string) error {
			return psqlerr.WithSeverity(psqlerr.WithCode(errors.New("unimplemented feature"), codes.FeatureNotSupported), psqlerr.LevelFatal)
		}
		return statement, nil, nil, nil
	}

	server, err := NewServer(handler, Logger(slogt.New(t)))
	assert.NoError(t, err)

	address := TListenAndServe(t, server)

	t.Run("lib/pq", func(t *testing.T) {
		connstr := fmt.Sprintf("host=%s port=%d sslmode=disable", address.IP, address.Port)
		conn, err := sql.Open("postgres", connstr)
		assert.NoError(t, err)

		_, err = conn.Query("SELECT *;")
		assert.Error(t, err)

		err = conn.Close()
		assert.NoError(t, err)
	})

	t.Run("jackc/pgx", func(t *testing.T) {
		ctx := context.Background()
		connstr := fmt.Sprintf("postgres://%s:%d", address.IP, address.Port)
		conn, err := pgx.Connect(ctx, connstr)
		assert.NoError(t, err)

		rows, _ := conn.Query(ctx, "SELECT *;")
		rows.Close()
		assert.Error(t, rows.Err())

		err = conn.Close(ctx)
		assert.NoError(t, err)
	})
}
