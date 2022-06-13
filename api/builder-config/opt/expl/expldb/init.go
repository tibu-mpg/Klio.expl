package expldb

import (
	"context"
	"embed"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"klio/expl/util"
	"time"
)

func Init(databaseURL string) (*ExplDB, error) {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	waitUntilAvailable(db)
	err = applyMigrations(db)
	if err != nil {
		return nil, err
	}

	return &ExplDB{
		db: db,
	}, nil
}

func (e *ExplDB) Close() error {
	return e.db.Close()
}

func waitUntilAvailable(db *sqlx.DB) {
	for db.Ping() != nil {
		logrus.Info("Waiting for database...")
		time.Sleep(time.Second)
	}
}

//go:embed migrations/*.sql
var fs embed.FS

func applyMigrations(db *sqlx.DB) (err error) {
	srcDrv, err := iofs.New(fs, "migrations")
	if err != nil {
		return err
	}
	defer util.CloseAndAppendError(srcDrv, &err)

	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer util.CloseAndAppendError(conn, &err)

	// postgres.WithInstance(...) is not used here, because its *sql.Conn cannot be closed without closing the *sql.DB
	dbDrv, err := postgres.WithConnection(context.Background(), conn, &postgres.Config{})
	if err != nil {
		return err
	}
	// dbDrv.Close() is not deferred here because that would call conn.Close() again and fail

	mig, err := migrate.NewWithInstance("iofs", srcDrv, "postgres", dbDrv)
	if err != nil {
		return err
	}
	// mig.Close() is not deferred here because that would call dbDrv.Close(), which woul call conn.Close() and fail

	mig.Log = &migrateLoggerAdapter{}

	err = mig.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}

	return nil
}

type migrateLoggerAdapter struct {
}

func (r *migrateLoggerAdapter) Printf(format string, v ...interface{}) {
	logrus.Infof(format, v...)
}

func (r *migrateLoggerAdapter) Verbose() bool {
	return true
}
