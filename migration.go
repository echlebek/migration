package migration

import (
	"database/sql"
	"fmt"
)

var ef = fmt.Errorf

// GetVersion is any function that can retrieve the migration version of a
// particular database. It is exposed in case a client wants to override the
// default behavior of this package. (For example, by using the `user_version`
// PRAGMA in SQLite.)
//
// The DefaultGetVersion function provided with this package creates its own
// table with a single column and a single row.
//
// The version returned should be equivalent to the number of migrations
// applied to this database. It should be 0 if no migrations have been applied
// yet.
//
// If an error is returned, the migration automatically fails.
type GetVersion func(*sql.Tx) (int, error)

// The default way to get the version from a database. If the database has
// had no migrations performed, then it creates a table with a single row and
// a single column storing the version as 0. It then returns 0.
//
// If the table exists, then the version stored in the table is returned.
var DefaultGetVersion GetVersion = defaultGetVersion

// SetVersion is the dual of GetVersion. It allows the client to define a
// different mechanism for setting the database version than the one used by
// DefaultSetVersion in this package.
//
// If an error is returned, the migration that tried to set the version
// automatically fails.
type SetVersion func(*sql.Tx, int) error

// The default way to set the version of the database. If the database has had
// no migrations performed, then it creates a table with a single row and a
// single column and storing the version given there.
//
// If the table exists, then the existing version is overwritten.
var DefaultSetVersion SetVersion = defaultSetVersion

// Migrator corresponds to a function that updates the database by one version.
// Note that a migration should NOT call Rollback or Commit. Instead, this
// package will call Rollback for you if your migration returns an error. If
// no error is returned, then Commit is called.
type Migrator func(*sql.Tx) error

// Open wraps the Open function from the database/sql function, but performs
// a series of migrations on a database if they haven't been performed already.
//
// Migrations are tracked by a simple versioning scheme. The version of the
// database is the number of migrations that have been performed on it.
// Similarly, the version of your library is the number of migrations that are
// defined.
//
// If Open returns successfully, then the database and your library will have
// the same versions. If there was a problem migrating---or if the database
// version is greater than your library version---then an error is returned.
// Since all migrations are performed in a single transaction, if an error
// occurs, no changes are made to the database. (Assuming you're using a
// relational database that allows modifications to a schema to be rolled back.)
//
// Note that this versioning scheme includes no semantic analysis. It is up to
// client to ensure that once a migration is defined, it never changes.
//
// The details of how the version is stored are opaque to the client, but in
// general, it will add a table to your database called "migration_version"
// with a single column containing a single row.
func Open(driver, dsn string, migrations []Migrator) (*sql.DB, error) {
	return OpenWith(driver, dsn, migrations, nil, nil)
}

// OpenWith is exactly like Open, except it allows the client to specify their
// own versioning scheme. Note that versionGet and versionSet must BOTH be
// nil or BOTH be non-nil. Otherwise, this function panics. This because the
// implementation of one generally relies on the implementation of the other.
//
// If versionGet and versionSet are both set to nil, then the behavior of this
// function is identical to the behavior of Open.
func OpenWith(
	driver, dsn string,
	migrations []Migrator,
	versionGet GetVersion,
	versionSet SetVersion,
) (*sql.DB, error) {
	if (versionGet == nil && versionSet != nil) ||
		(versionGet != nil && versionSet == nil) {
		panic("versionGet/versionSet must both be nil or both be non-nil")
	}
	if versionGet == nil {
		versionGet = DefaultGetVersion
	}
	if versionSet == nil {
		versionSet = DefaultSetVersion
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	m := migration{db, migrations, versionGet, versionSet}
	if err := m.migrate(); err != nil {
		return nil, err
	}
	return db, nil
}

type migration struct {
	*sql.DB
	migrations []Migrator
	getVersion GetVersion
	setVersion SetVersion
}

func (m migration) migrate() error {
	tx, err := m.Begin()
	if err != nil {
		return ef("Could not start transaction: %s", err)
	}
	libVersion := len(m.migrations)
	dbVersion, err := m.getVersion(tx)
	if err != nil {
		return ef("Could not get DB version: %s", err)
	}
	if dbVersion > libVersion {
		return ef("Database version (%d) is greater than library version (%d).",
			dbVersion, libVersion)
	}
	if dbVersion == libVersion {
		return tx.Commit()
	}

	for i := dbVersion; i < libVersion; i++ {
		if err := m.migrations[i](tx); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return ef(
					"When migrating from %d to %d, got error '%s' and "+
						"got error '%s' after trying to rollback.",
					i, i+1, err, err2)
			}
			return ef(
				"When migrating from %d to %d, got error '%s' and "+
					"successfully rolled back.", i, i+1, err)
		}
	}
	if err := m.setVersion(tx, libVersion); err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return ef(
				"When trying to set version to %d (from %d), got error '%s' "+
					"and got error '%s' after trying to rollback.",
				libVersion, dbVersion, err, err2)
		}
		return ef(
			"When trying to set version to %d (from %d), got error '%s' "+
				"and successfully rolled back.",
			libVersion, dbVersion, err)
	}
	if err := tx.Commit(); err != nil {
		return ef("Error committing migration from %d to %d: %s",
			dbVersion, libVersion, err)
	}
	return nil
}

func defaultGetVersion(tx *sql.Tx) (int, error) {
	v, err := getVersion(tx)
	if err != nil {
		if err := createVersionTable(tx); err != nil {
			return 0, err
		}
		return getVersion(tx)
	}
	return v, nil
}

func defaultSetVersion(tx *sql.Tx, version int) error {
	if err := setVersion(tx, version); err != nil {
		if err := createVersionTable(tx); err != nil {
			return err
		}
		return setVersion(tx, version)
	}
	return nil
}

func getVersion(tx *sql.Tx) (int, error) {
	var version int
	r := tx.QueryRow("SELECT version FROM migration_version")
	if err := r.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func setVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec("UPDATE migration_version SET version = $1", version)
	return err
}

func createVersionTable(tx *sql.Tx) error {
	_, err := tx.Exec(`
		CREATE TABLE migration_version (
			version INTEGER
		);
		INSERT INTO migration_version (version) VALUES (0)`)
	return err
}