package wire

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jeroenrinzema/psql-wire/pkg/buffer"
	"github.com/jeroenrinzema/psql-wire/pkg/mock"
	"github.com/jeroenrinzema/psql-wire/pkg/types"
	"github.com/lib/pq/oid"
	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/assert"
)

func TestMessageSizeExceeded(t *testing.T) {
	server, err := NewServer(nil, Logger(slogt.New(t)))
	if err != nil {
		t.Fatal(err)
	}

	address := TListenAndServe(t, server)
	conn, err := net.Dial("tcp", address.String())
	if err != nil {
		t.Fatal(err)
	}

	client := mock.NewClient(t, conn)
	client.Handshake(t)
	client.Authenticate(t)
	client.ReadyForQuery(t)

	// NOTE: attempt to send a message twice the max buffer size
	size := uint32(buffer.DefaultBufferSize * 2)
	t.Logf("writing message of size: %d", size)

	client.Start(types.ClientSimpleQuery)
	client.AddBytes(make([]byte, size))
	err = client.End()
	if err != nil {
		t.Fatal(err)
	}

	client.Error(t)
	client.Close(t)
}

func TestBindMessageParameters(t *testing.T) {
	t.Parallel()

	columns := Columns{
		{
			Table:  0,
			Name:   "full_name",
			Oid:    oid.T_text,
			Width:  256,
			Format: TextFormat,
		},
		{
			Table:  0,
			Name:   "answer_to_life_the_universe_and_everything",
			Oid:    oid.T_text,
			Width:  256,
			Format: TextFormat,
		},
	}

	handler := func(ctx context.Context, query string) (PreparedStatementFn, []oid.Oid, Columns, error) {
		statement := func(ctx context.Context, writer DataWriter, parameters []string) error {
			t.Log("serving query")

			if len(parameters) != 2 {
				return fmt.Errorf("unexpected amount of parameters %d, expected 2", len(parameters))
			}

			writer.Row([]any{parameters[0], parameters[1]}) //nolint:errcheck
			return writer.Complete("SELECT 1")
		}

		return statement, ParseParameters(query), columns, nil
	}

	server, err := NewServer(handler, Logger(slogt.New(t)))
	if err != nil {
		t.Fatal(err)
	}

	address := TListenAndServe(t, server)

	ctx := context.Background()
	connstr := fmt.Sprintf("postgres://%s:%d", address.IP, address.Port)

	t.Run("pgx", func(t *testing.T) {
		conn, err := pgx.Connect(ctx, connstr)
		if err != nil {
			t.Fatal(err)
		}

		defer conn.Close(ctx)

		rows, err := conn.Query(ctx, "SELECT $1 $2;", "John Doe", 42)
		if err != nil {
			t.Fatal(err)
		}

		assert.True(t, rows.Next())

		var name string
		var answer string

		err = rows.Scan(&name, &answer)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("scan result: %s, %s", name, answer)

		assert.Equal(t, name, "John Doe")
		assert.Equal(t, answer, "42")

		assert.False(t, rows.Next())

		rows.Close()

		err = conn.Close(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})

}
