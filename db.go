// Package database is a library help for interact with database by model
package gomodel

import "database/sql"

type (
	// DB holds database connection, all typeinfos, and sql cache
	DB struct {
		// driver string
		*sql.DB
		tables map[string]*Table
		cache

		// initial models count for 'All'
		InitialModels int
	}
)

// Open create a database manager and connect to database server
func Open(driver, dsn string, maxIdle, maxOpen int) (*DB, error) {
	db := NewDB()
	err := db.Connect(driver, dsn, maxIdle, maxOpen)

	return db, err
}

// New create a new db structure
func NewDB() *DB {
	return &DB{
		tables:        make(map[string]*Table),
		InitialModels: 10,
	}
}

// Connect to database server
func (db *DB) Connect(driver, dsn string, maxIdle, maxOpen int) error {
	db_, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}

	db_.SetMaxIdleConns(maxIdle)
	db_.SetMaxOpenConns(maxOpen)
	db.DB = db_
	db.cache = newCache()

	return nil
}

// register save table of model
func (db *DB) register(model Model, table string) *Table {
	t := parse(model, db)
	db.tables[table] = t

	return t
}

// Table return infomation of given model
// if table not exist, do parse and save it
func (db *DB) Table(model Model) *Table {
	table := model.Table()
	if t, has := db.tables[table]; has {
		return t
	}

	return db.register(model, table)
}

func (db *DB) Insert(model Model, fields uint64, resType ResultType) (int64, error) {
	return db.ArgsInsert(model, fields, resType, FieldVals(model, fields)...)
}

func (db *DB) ArgsInsert(model Model, fields uint64, resType ResultType, args ...interface{}) (int64, error) {
	stmt, err := db.Table(model).StmtInsert(db.DB, fields)

	return Exec(stmt, err, resType, args...)
}

func (db *DB) Update(model Model, fields, whereFields uint64) (int64, error) {
	c1, c2 := NumFields(fields), NumFields(whereFields)
	args := make([]interface{}, c1+c2)
	model.Vals(fields, args)
	model.Vals(whereFields, args[c1:])

	return db.ArgsUpdate(model, fields, whereFields, args...)
}

func (db *DB) ArgsUpdate(model Model, fields, whereFields uint64, args ...interface{}) (int64, error) {
	stmt, err := db.Table(model).StmtUpdate(db.DB, fields, whereFields)

	return Update(stmt, err, args...)
}

func (db *DB) Delete(model Model, whereFields uint64) (int64, error) {
	return db.ArgsDelete(model, whereFields, FieldVals(model, whereFields)...)
}

func (db *DB) ArgsDelete(model Model, whereFields uint64, args ...interface{}) (int64, error) {
	stmt, err := db.Table(model).StmtDelete(db.DB, whereFields)

	return Update(stmt, err, args...)
}

// One select one row from database
func (db *DB) One(model Model, fields, whereFields uint64) error {
	stmt, err := db.Table(model).StmtOne(db.DB, fields, whereFields)
	scanner := Query(stmt, err, FieldVals(model, whereFields)...)

	return scanner.One(FieldPtrs(model, fields)...)
}

func (db *DB) Limit(store Store, model Model, fields, whereFields uint64, start, count int) error {
	args := FieldVals(model, whereFields, start, count)

	return db.ArgsLimit(store, model, fields, whereFields, args...)
}

// The last two arguments must be "start" and "count" of limition with type "int"
func (db *DB) ArgsLimit(store Store, model Model, fields, whereFields uint64, args ...interface{}) error {
	stmt, err := db.Table(model).StmtLimit(db.DB, fields, whereFields)
	scanner := Query(stmt, err, args...)

	return scanner.Limit(store, args[len(args)-1].(int))
}

func (db *DB) All(store Store, model Model, fields, whereFields uint64) error {
	return db.ArgsAll(store, model, fields, whereFields, FieldVals(model, whereFields)...)
}

// ArgsAll select all  the last two argument must be "start" and "count"
func (db *DB) ArgsAll(store Store, model Model, fields, whereFields uint64, args ...interface{}) error {
	stmt, err := db.Table(model).StmtAll(db.DB, fields, whereFields)
	scanner := Query(stmt, err, args...)

	return scanner.All(store, db.InitialModels)
}

// Count return count of rows for model, arguments was extracted from Model
func (db *DB) Count(model Model, whereFields uint64) (count int64, err error) {
	return db.ArgsCount(model, whereFields, FieldVals(model, whereFields)...)
}

// ArgsCount return count of rows for model use custome arguments
func (db *DB) ArgsCount(model Model, whereFields uint64, args ...interface{}) (count int64, err error) {
	t := db.Table(model)

	stmt, err := t.StmtCount(db.DB, whereFields)
	scanner := Query(stmt, err, args...)

	err = scanner.One(&count)

	return
}

func (db *DB) IncrBy(model Model, field, whereFields uint64, count int) (int64, error) {
	args := make([]interface{}, NumFields(whereFields)+1)
	args[0] = count
	model.Vals(whereFields, args[1:])

	return db.ArgsIncrBy(model, field, whereFields, args...)
}

func (db *DB) ArgsIncrBy(model Model, field, whereFields uint64, args ...interface{}) (int64, error) {
	stmt, err := db.Table(model).StmtIncrBy(db.DB, field, whereFields)

	return Update(stmt, err, args...)
}

// ExecUpdate execute a update operation, return resolved result
func (db *DB) ExecUpdate(sql string, args ...interface{}) (int64, error) {
	return db.Exec(sql, RES_ROWS, args...)
}

// Exec execute a update operation, return resolved result
func (db *DB) Exec(sql string, resType ResultType, args ...interface{}) (int64, error) {
	res, err := db.DB.Exec(sql, args...)

	return ResolveResult(res, err, resType)
}

func (db *DB) ExecById(idsql IdSql, resTyp ResultType, args ...interface{}) (int64, error) {
	stmt, err := db.StmtById(db, idsql)

	return Exec(stmt, err, resTyp, args...)
}

func (db *DB) UpdateById(idsql IdSql, args ...interface{}) (int64, error) {
	return db.ExecById(idsql, RES_ROWS, args...)
}

func (db *DB) QueryById(idsql IdSql, args ...interface{}) Scanner {
	stmt, err := db.StmtById(db, idsql)

	return Query(stmt, err, args...)
}

var emptyTX = Tx{}

func (db *DB) Begin() (Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return emptyTX, err
	}

	return Tx{
		Tx: tx,
		db: db,
	}, nil
}
