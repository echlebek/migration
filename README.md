This is a fork of github.com/BurntSushi/migration that's written to work with
github.com/jackc/pgx instead of database/sql. It only works with PostgresQL.

Package migration for Golang automatically handles versioning of a database 
schema by applying a series of migrations supplied by the client. It uses 
features only from the database/sql package, so it tries to be driver 
independent. However, to track the version of the database, it is necessary to 
execute some SQL. I've made an effort to keep those queries simple, but if they 
don't work with your database, you may override them.

This package works by applying a series of migrations to a database. Once a 
migration is created, it should never be changed. Every time a database is 
opened with this package, all necessary migrations are executed in a single 
transaction. If any part of the process fails, an error is returned and the 
transaction is rolled back so that the database is left untouched.

The version of a database is defined as the number of migrations applied to it.


### Installation

If you have Go installed, then  `migration` can be installed with `go get`:

    go get github.com/echlebek/migration


### Documentation

Documentation is available at
[godoc.org/github.com/echlebek/migration](http://godoc.org/github.com/echlebek/migration).


### Unstable

This is a fork of a project that was released with the caveat that it is unstable.
That was 8 years ago, so it seems BurntSushi has been happy with the interface. :)
This fork of the library should be considered *more* unstable.
