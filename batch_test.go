package pgx_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
)

func TestConnSendBatch(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	sql := `create temporary table ledger(
	  id serial primary key,
	  description varchar not null,
	  amount int not null
	);`
	mustExec(t, conn, sql)

	batch := &pgx.Batch{}
	batch.Queue("insert into ledger(description, amount) values($1, $2)",
		[]interface{}{"q1", 1},
		[]uint32{pgtype.VarcharOID, pgtype.Int4OID},
		nil,
	)
	batch.Queue("insert into ledger(description, amount) values($1, $2)",
		[]interface{}{"q2", 2},
		[]uint32{pgtype.VarcharOID, pgtype.Int4OID},
		nil,
	)
	batch.Queue("insert into ledger(description, amount) values($1, $2)",
		[]interface{}{"q3", 3},
		[]uint32{pgtype.VarcharOID, pgtype.Int4OID},
		nil,
	)
	batch.Queue("select id, description, amount from ledger order by id",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode, pgx.TextFormatCode, pgx.BinaryFormatCode},
	)
	batch.Queue("select sum(amount) from ledger",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)

	br := conn.SendBatch(context.Background(), batch)

	ct, err := br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 1 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
	}

	ct, err = br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 1 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
	}

	ct, err = br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 1 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
	}

	rows, err := br.QueryResults()
	if err != nil {
		t.Error(err)
	}

	var id int32
	var description string
	var amount int32
	if !rows.Next() {
		t.Fatal("expected a row to be available")
	}
	if err := rows.Scan(&id, &description, &amount); err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Errorf("id => %v, want %v", id, 1)
	}
	if description != "q1" {
		t.Errorf("description => %v, want %v", description, "q1")
	}
	if amount != 1 {
		t.Errorf("amount => %v, want %v", amount, 1)
	}

	if !rows.Next() {
		t.Fatal("expected a row to be available")
	}
	if err := rows.Scan(&id, &description, &amount); err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Errorf("id => %v, want %v", id, 2)
	}
	if description != "q2" {
		t.Errorf("description => %v, want %v", description, "q2")
	}
	if amount != 2 {
		t.Errorf("amount => %v, want %v", amount, 2)
	}

	if !rows.Next() {
		t.Fatal("expected a row to be available")
	}
	if err := rows.Scan(&id, &description, &amount); err != nil {
		t.Fatal(err)
	}
	if id != 3 {
		t.Errorf("id => %v, want %v", id, 3)
	}
	if description != "q3" {
		t.Errorf("description => %v, want %v", description, "q3")
	}
	if amount != 3 {
		t.Errorf("amount => %v, want %v", amount, 3)
	}

	if rows.Next() {
		t.Fatal("did not expect a row to be available")
	}

	if rows.Err() != nil {
		t.Fatal(rows.Err())
	}

	err = br.QueryRowResults().Scan(&amount)
	if err != nil {
		t.Error(err)
	}
	if amount != 6 {
		t.Errorf("amount => %v, want %v", amount, 6)
	}

	err = br.Close()
	if err != nil {
		t.Fatal(err)
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchWithPreparedStatement(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	_, err := conn.Prepare(context.Background(), "ps1", "select n from generate_series(0,$1::int) n")
	if err != nil {
		t.Fatal(err)
	}

	batch := &pgx.Batch{}

	queryCount := 3
	for i := 0; i < queryCount; i++ {
		batch.Queue("ps1",
			[]interface{}{5},
			nil,
			nil,
		)
	}

	br := conn.SendBatch(context.Background(), batch)

	for i := 0; i < queryCount; i++ {
		rows, err := br.QueryResults()
		if err != nil {
			t.Fatal(err)
		}

		for k := 0; rows.Next(); k++ {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != k {
				t.Fatalf("n => %v, want %v", n, k)
			}
		}

		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
	}

	err = br.Close()
	if err != nil {
		t.Fatal(err)
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchCloseRowsPartiallyRead(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	batch := &pgx.Batch{}
	batch.Queue("select n from generate_series(0,5) n",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)
	batch.Queue("select n from generate_series(0,5) n",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)

	br := conn.SendBatch(context.Background(), batch)

	rows, err := br.QueryResults()
	if err != nil {
		t.Error(err)
	}

	for i := 0; i < 3; i++ {
		if !rows.Next() {
			t.Error("expected a row to be available")
		}

		var n int
		if err := rows.Scan(&n); err != nil {
			t.Error(err)
		}
		if n != i {
			t.Errorf("n => %v, want %v", n, i)
		}
	}

	rows.Close()

	rows, err = br.QueryResults()
	if err != nil {
		t.Error(err)
	}

	for i := 0; rows.Next(); i++ {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Error(err)
		}
		if n != i {
			t.Errorf("n => %v, want %v", n, i)
		}
	}

	if rows.Err() != nil {
		t.Error(rows.Err())
	}

	err = br.Close()
	if err != nil {
		t.Fatal(err)
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchQueryError(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	batch := &pgx.Batch{}
	batch.Queue("select n from generate_series(0,5) n where 100/(5-n) > 0",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)
	batch.Queue("select n from generate_series(0,5) n",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)

	br := conn.SendBatch(context.Background(), batch)

	rows, err := br.QueryResults()
	if err != nil {
		t.Error(err)
	}

	for i := 0; rows.Next(); i++ {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Error(err)
		}
		if n != i {
			t.Errorf("n => %v, want %v", n, i)
		}
	}

	if pgErr, ok := rows.Err().(*pgconn.PgError); !(ok && pgErr.Code == "22012") {
		t.Errorf("rows.Err() => %v, want error code %v", rows.Err(), 22012)
	}

	err = br.Close()
	if pgErr, ok := err.(*pgconn.PgError); !(ok && pgErr.Code == "22012") {
		t.Errorf("rows.Err() => %v, want error code %v", err, 22012)
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchQuerySyntaxError(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	batch := &pgx.Batch{}
	batch.Queue("select 1 1",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)

	br := conn.SendBatch(context.Background(), batch)

	var n int32
	err := br.QueryRowResults().Scan(&n)
	if pgErr, ok := err.(*pgconn.PgError); !(ok && pgErr.Code == "42601") {
		t.Errorf("rows.Err() => %v, want error code %v", err, 42601)
	}

	err = br.Close()
	if err == nil {
		t.Error("Expected error")
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchQueryRowInsert(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	sql := `create temporary table ledger(
	  id serial primary key,
	  description varchar not null,
	  amount int not null
	);`
	mustExec(t, conn, sql)

	batch := &pgx.Batch{}
	batch.Queue("select 1",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)
	batch.Queue("insert into ledger(description, amount) values($1, $2),($1, $2)",
		[]interface{}{"q1", 1},
		[]uint32{pgtype.VarcharOID, pgtype.Int4OID},
		nil,
	)

	br := conn.SendBatch(context.Background(), batch)

	var value int
	err := br.QueryRowResults().Scan(&value)
	if err != nil {
		t.Error(err)
	}

	ct, err := br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 2 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 2)
	}

	br.Close()

	ensureConnValid(t, conn)
}

func TestConnSendBatchQueryPartialReadInsert(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	sql := `create temporary table ledger(
	  id serial primary key,
	  description varchar not null,
	  amount int not null
	);`
	mustExec(t, conn, sql)

	batch := &pgx.Batch{}
	batch.Queue("select 1 union all select 2 union all select 3",
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)
	batch.Queue("insert into ledger(description, amount) values($1, $2),($1, $2)",
		[]interface{}{"q1", 1},
		[]uint32{pgtype.VarcharOID, pgtype.Int4OID},
		nil,
	)

	br := conn.SendBatch(context.Background(), batch)

	rows, err := br.QueryResults()
	if err != nil {
		t.Error(err)
	}
	rows.Close()

	ct, err := br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 2 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 2)
	}

	br.Close()

	ensureConnValid(t, conn)
}

func TestTxSendBatch(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	sql := `create temporary table ledger1(
	  id serial primary key,
	  description varchar not null
	);`
	mustExec(t, conn, sql)

	sql = `create temporary table ledger2(
	  id int primary key,
	  amount int not null
	);`
	mustExec(t, conn, sql)

	tx, _ := conn.Begin(context.Background())
	batch := &pgx.Batch{}
	batch.Queue("insert into ledger1(description) values($1) returning id",
		[]interface{}{"q1"},
		[]uint32{pgtype.VarcharOID},
		[]int16{pgx.BinaryFormatCode},
	)

	br := tx.SendBatch(context.Background(), batch)

	var id int
	err := br.QueryRowResults().Scan(&id)
	if err != nil {
		t.Error(err)
	}
	br.Close()

	batch = &pgx.Batch{}
	batch.Queue("insert into ledger2(id,amount) values($1, $2)",
		[]interface{}{id, 2},
		[]uint32{pgtype.Int4OID, pgtype.Int4OID},
		nil,
	)

	batch.Queue("select amount from ledger2 where id = $1",
		[]interface{}{id},
		[]uint32{pgtype.Int4OID},
		nil,
	)

	br = tx.SendBatch(context.Background(), batch)

	ct, err := br.ExecResults()
	if err != nil {
		t.Error(err)
	}
	if ct.RowsAffected() != 1 {
		t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
	}

	var amount int
	err = br.QueryRowResults().Scan(&amount)
	if err != nil {
		t.Error(err)
	}

	br.Close()
	tx.Commit(context.Background())

	var count int
	conn.QueryRow(context.Background(), "select count(1) from ledger1 where id = $1", id).Scan(&count)
	if count != 1 {
		t.Errorf("count => %v, want %v", count, 1)
	}

	err = br.Close()
	if err != nil {
		t.Fatal(err)
	}

	ensureConnValid(t, conn)
}

func TestTxSendBatchRollback(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	sql := `create temporary table ledger1(
	  id serial primary key,
	  description varchar not null
	);`
	mustExec(t, conn, sql)

	tx, _ := conn.Begin(context.Background())
	batch := &pgx.Batch{}
	batch.Queue("insert into ledger1(description) values($1) returning id",
		[]interface{}{"q1"},
		[]uint32{pgtype.VarcharOID},
		[]int16{pgx.BinaryFormatCode},
	)

	br := tx.SendBatch(context.Background(), batch)

	var id int
	err := br.QueryRowResults().Scan(&id)
	if err != nil {
		t.Error(err)
	}
	br.Close()
	tx.Rollback(context.Background())

	row := conn.QueryRow(context.Background(), "select count(1) from ledger1 where id = $1", id)
	var count int
	row.Scan(&count)
	if count != 0 {
		t.Errorf("count => %v, want %v", count, 0)
	}

	ensureConnValid(t, conn)
}

func TestConnBeginBatchDeferredError(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv("PGX_TEST_DATABASE"))
	defer closeConn(t, conn)

	mustExec(t, conn, `create temporary table t (
		id text primary key,
		n int not null,
		unique (n) deferrable initially deferred
	);

	insert into t (id, n) values ('a', 1), ('b', 2), ('c', 3);`)

	batch := &pgx.Batch{}

	batch.Queue(`update t set n=n+1 where id='b' returning *`,
		nil,
		nil,
		[]int16{pgx.BinaryFormatCode},
	)

	br := conn.SendBatch(context.Background(), batch)

	rows, err := br.QueryResults()
	if err != nil {
		t.Error(err)
	}

	for rows.Next() {
		var id string
		var n int32
		err = rows.Scan(&id, &n)
		if err != nil {
			t.Fatal(err)
		}
	}

	err = br.Close()
	if err == nil {
		t.Fatal("expected error 23505 but got none")
	}

	if err, ok := err.(*pgconn.PgError); !ok || err.Code != "23505" {
		t.Fatalf("expected error 23505, got %v", err)
	}

	ensureConnValid(t, conn)
}
