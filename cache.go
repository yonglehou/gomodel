package gomodel

import (
	"database/sql"
	"log"
	"sync/atomic"
)

type (
	// ID help for generate sql id
	ID int32

	// cacheItem keeps the sql and prepared statement of it
	cacheItem struct {
		sql  string
		stmt *sql.Stmt
	}

	Preparer interface {
		Prepare(sql string) (*sql.Stmt, error)
	}

	// Cacher store all the sql, statement by sql type and id
	// typically, the sql id of predefied sql type is
	// fields << numField( of Model) | whereFields,
	// it's used for a single model
	//
	// if custom is necessary, call cache.ExtendType(cache.Types()+1) to make
	// a new type, the sql id is bring your owns, also you can still use the standard
	// FieldIdentity(fields, whereFields) if possible
	Cacher struct {
		cache []map[uint]cacheItem // [type]map[id]{sql, stmt}
	}

	SQLPrinter func(string, ...interface{})
)

const (
	// These are five predefined sql types
	INSERT uint = iota
	DELETE
	UPDATE
	SELECT_LIMIT
	SELECT_ONE
	SELECT_ALL

	defaultTypeEnd
)

var (
	// Types defines the default sql types count, it's default applied to all
	// models.
	// Change it before register any models.
	Types                 = defaultTypeEnd
	sqlPrinter SQLPrinter = func(string, ...interface{}) {}
)

func (p SQLPrinter) Print(fromcache bool, sql string) {
	p("Cached: %t, SQL: %s\n", fromcache, sql)
}

// SQLPrint enable sql print for each operation
func SQLPrint(enable bool, printer func(formart string, v ...interface{})) {
	if !enable {
		return
	}

	sqlPrinter = printer
	if sqlPrinter == nil {
		sqlPrinter = log.Printf
	}
}

// NewID create a id generator used for StmtById, normally, one ID is enough,
// it's safety used for all models
//
// Example:
// sqlid := gomodel.NewID
//
// sqlidUserUpdate := slqid.New()
// sqlidUserDelete := sqlid.New()
// sqlidBookUpdate := sqlid.New()
// sqlidBookDelete := sqlid.New()
//
func NewID() *ID {
	var i int32

	return (*ID)(&i)
}

// New generate a new sql id
func (i *ID) New() uint {
	return uint(atomic.AddInt32((*int32)(i), 1))
}

// NewCacher create a common sql and statement cacher with given types count
// this will make no parameter checks
//
// the DB instance and each TypeInfo already embed a Cacher, typically, it's not
// necessary to call this
func NewCacher(types uint) Cacher {
	c := Cacher{
		cache: make([]map[uint]cacheItem, types),
	}

	for i := uint(0); i < types; i++ {
		c.cache[i] = make(map[uint]cacheItem)
	}

	return c
}

// ExtendType typically used to extend types of Cacher, but it also can be used
// to shrink the cacher, return value will be the new types count you passed in
//
// Example:
// //a.go
// newType1 := c.ExtendType(c.Types()+1)
// //b.go
// newType2 := c.ExtendType(c.Types()+1)
func (c *Cacher) ExtendType(typ uint) uint {
	if l := uint(len(c.cache)); typ > l {
		cache := make([]map[uint]cacheItem, typ)
		copy(cache, c.cache)

		for ; l < typ; l++ {
			cache[l] = make(map[uint]cacheItem)
		}
		c.cache = cache
	} else {
		c.cache = c.cache[:typ]
	}

	return typ - 1
}

// StmtById search a prepared statement for given sql type by id, if not found,
// create with the creator, and prepared the sql to a statement, cache it, then
// return
func (c *Cacher) StmtById(p Preparer, typ, id uint, create func() string) (*sql.Stmt, error) {
	if item, has := c.cache[typ][id]; has {
		sqlPrinter.Print(true, item.sql)

		return item.stmt, nil
	}

	sql_ := create()
	sqlPrinter.Print(false, sql_)

	stmt, err := p.Prepare(sql_)
	if err != nil {
		return nil, err
	}

	c.cache[typ][id] = cacheItem{sql: sql_, stmt: stmt}

	return stmt, nil
}

// Types return the sql types count of current Cacher
func (c *Cacher) Types() uint {
	return uint(len(c.cache))
}

// GetStmt get sql and statement from cacher, if not found, "" and nil was returned
func (c *Cacher) GetStmt(typ, id uint) (string, *sql.Stmt) {
	item, has := c.cache[typ][id]
	if !has {
		return "", nil
	}

	return item.sql, item.stmt
}

// SetStmt prepare a sql to statement, cache then return it
func (c *Cacher) SetStmt(p Preparer, typ uint, id uint, sql string) (*sql.Stmt, error) {
	stmt, err := p.Prepare(sql)
	if err != nil {
		return nil, err
	}

	c.cache[typ][id] = cacheItem{
		sql:  sql,
		stmt: stmt,
	}

	return stmt, nil
}

func (c *Cacher) PrepareStmt(p Preparer, typ, id uint) (string, *sql.Stmt, error) {
	item, has := c.cache[typ][id]
	if !has {
		return "", nil, nil
	}

	stmt, err := p.Prepare(item.sql)
	return item.sql, stmt, err
}
