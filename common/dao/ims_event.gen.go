// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.
// Code generated by gorm.io/gen. DO NOT EDIT.

package dao

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"gorm.io/gen"
	"gorm.io/gen/field"

	"gorm.io/plugin/dbresolver"

	"github.com/donknap/dpanel/common/entity"
)

func newEvent(db *gorm.DB, opts ...gen.DOOption) event {
	_event := event{}

	_event.eventDo.UseDB(db, opts...)
	_event.eventDo.UseModel(&entity.Event{})

	tableName := _event.eventDo.TableName()
	_event.ALL = field.NewAsterisk(tableName)
	_event.ID = field.NewInt32(tableName, "id")
	_event.Type = field.NewString(tableName, "type")
	_event.Action = field.NewString(tableName, "action")
	_event.Message = field.NewString(tableName, "message")
	_event.CreatedAt = field.NewString(tableName, "created_at")

	_event.fillFieldMap()

	return _event
}

type event struct {
	eventDo

	ALL       field.Asterisk
	ID        field.Int32
	Type      field.String
	Action    field.String
	Message   field.String
	CreatedAt field.String

	fieldMap map[string]field.Expr
}

func (e event) Table(newTableName string) *event {
	e.eventDo.UseTable(newTableName)
	return e.updateTableName(newTableName)
}

func (e event) As(alias string) *event {
	e.eventDo.DO = *(e.eventDo.As(alias).(*gen.DO))
	return e.updateTableName(alias)
}

func (e *event) updateTableName(table string) *event {
	e.ALL = field.NewAsterisk(table)
	e.ID = field.NewInt32(table, "id")
	e.Type = field.NewString(table, "type")
	e.Action = field.NewString(table, "action")
	e.Message = field.NewString(table, "message")
	e.CreatedAt = field.NewString(table, "created_at")

	e.fillFieldMap()

	return e
}

func (e *event) GetFieldByName(fieldName string) (field.OrderExpr, bool) {
	_f, ok := e.fieldMap[fieldName]
	if !ok || _f == nil {
		return nil, false
	}
	_oe, ok := _f.(field.OrderExpr)
	return _oe, ok
}

func (e *event) fillFieldMap() {
	e.fieldMap = make(map[string]field.Expr, 5)
	e.fieldMap["id"] = e.ID
	e.fieldMap["type"] = e.Type
	e.fieldMap["action"] = e.Action
	e.fieldMap["message"] = e.Message
	e.fieldMap["created_at"] = e.CreatedAt
}

func (e event) clone(db *gorm.DB) event {
	e.eventDo.ReplaceConnPool(db.Statement.ConnPool)
	return e
}

func (e event) replaceDB(db *gorm.DB) event {
	e.eventDo.ReplaceDB(db)
	return e
}

type eventDo struct{ gen.DO }

type IEventDo interface {
	gen.SubQuery
	Debug() IEventDo
	WithContext(ctx context.Context) IEventDo
	WithResult(fc func(tx gen.Dao)) gen.ResultInfo
	ReplaceDB(db *gorm.DB)
	ReadDB() IEventDo
	WriteDB() IEventDo
	As(alias string) gen.Dao
	Session(config *gorm.Session) IEventDo
	Columns(cols ...field.Expr) gen.Columns
	Clauses(conds ...clause.Expression) IEventDo
	Not(conds ...gen.Condition) IEventDo
	Or(conds ...gen.Condition) IEventDo
	Select(conds ...field.Expr) IEventDo
	Where(conds ...gen.Condition) IEventDo
	Order(conds ...field.Expr) IEventDo
	Distinct(cols ...field.Expr) IEventDo
	Omit(cols ...field.Expr) IEventDo
	Join(table schema.Tabler, on ...field.Expr) IEventDo
	LeftJoin(table schema.Tabler, on ...field.Expr) IEventDo
	RightJoin(table schema.Tabler, on ...field.Expr) IEventDo
	Group(cols ...field.Expr) IEventDo
	Having(conds ...gen.Condition) IEventDo
	Limit(limit int) IEventDo
	Offset(offset int) IEventDo
	Count() (count int64, err error)
	Scopes(funcs ...func(gen.Dao) gen.Dao) IEventDo
	Unscoped() IEventDo
	Create(values ...*entity.Event) error
	CreateInBatches(values []*entity.Event, batchSize int) error
	Save(values ...*entity.Event) error
	First() (*entity.Event, error)
	Take() (*entity.Event, error)
	Last() (*entity.Event, error)
	Find() ([]*entity.Event, error)
	FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*entity.Event, err error)
	FindInBatches(result *[]*entity.Event, batchSize int, fc func(tx gen.Dao, batch int) error) error
	Pluck(column field.Expr, dest interface{}) error
	Delete(...*entity.Event) (info gen.ResultInfo, err error)
	Update(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	Updates(value interface{}) (info gen.ResultInfo, err error)
	UpdateColumn(column field.Expr, value interface{}) (info gen.ResultInfo, err error)
	UpdateColumnSimple(columns ...field.AssignExpr) (info gen.ResultInfo, err error)
	UpdateColumns(value interface{}) (info gen.ResultInfo, err error)
	UpdateFrom(q gen.SubQuery) gen.Dao
	Attrs(attrs ...field.AssignExpr) IEventDo
	Assign(attrs ...field.AssignExpr) IEventDo
	Joins(fields ...field.RelationField) IEventDo
	Preload(fields ...field.RelationField) IEventDo
	FirstOrInit() (*entity.Event, error)
	FirstOrCreate() (*entity.Event, error)
	FindByPage(offset int, limit int) (result []*entity.Event, count int64, err error)
	ScanByPage(result interface{}, offset int, limit int) (count int64, err error)
	Scan(result interface{}) (err error)
	Returning(value interface{}, columns ...string) IEventDo
	UnderlyingDB() *gorm.DB
	schema.Tabler
}

func (e eventDo) Debug() IEventDo {
	return e.withDO(e.DO.Debug())
}

func (e eventDo) WithContext(ctx context.Context) IEventDo {
	return e.withDO(e.DO.WithContext(ctx))
}

func (e eventDo) ReadDB() IEventDo {
	return e.Clauses(dbresolver.Read)
}

func (e eventDo) WriteDB() IEventDo {
	return e.Clauses(dbresolver.Write)
}

func (e eventDo) Session(config *gorm.Session) IEventDo {
	return e.withDO(e.DO.Session(config))
}

func (e eventDo) Clauses(conds ...clause.Expression) IEventDo {
	return e.withDO(e.DO.Clauses(conds...))
}

func (e eventDo) Returning(value interface{}, columns ...string) IEventDo {
	return e.withDO(e.DO.Returning(value, columns...))
}

func (e eventDo) Not(conds ...gen.Condition) IEventDo {
	return e.withDO(e.DO.Not(conds...))
}

func (e eventDo) Or(conds ...gen.Condition) IEventDo {
	return e.withDO(e.DO.Or(conds...))
}

func (e eventDo) Select(conds ...field.Expr) IEventDo {
	return e.withDO(e.DO.Select(conds...))
}

func (e eventDo) Where(conds ...gen.Condition) IEventDo {
	return e.withDO(e.DO.Where(conds...))
}

func (e eventDo) Order(conds ...field.Expr) IEventDo {
	return e.withDO(e.DO.Order(conds...))
}

func (e eventDo) Distinct(cols ...field.Expr) IEventDo {
	return e.withDO(e.DO.Distinct(cols...))
}

func (e eventDo) Omit(cols ...field.Expr) IEventDo {
	return e.withDO(e.DO.Omit(cols...))
}

func (e eventDo) Join(table schema.Tabler, on ...field.Expr) IEventDo {
	return e.withDO(e.DO.Join(table, on...))
}

func (e eventDo) LeftJoin(table schema.Tabler, on ...field.Expr) IEventDo {
	return e.withDO(e.DO.LeftJoin(table, on...))
}

func (e eventDo) RightJoin(table schema.Tabler, on ...field.Expr) IEventDo {
	return e.withDO(e.DO.RightJoin(table, on...))
}

func (e eventDo) Group(cols ...field.Expr) IEventDo {
	return e.withDO(e.DO.Group(cols...))
}

func (e eventDo) Having(conds ...gen.Condition) IEventDo {
	return e.withDO(e.DO.Having(conds...))
}

func (e eventDo) Limit(limit int) IEventDo {
	return e.withDO(e.DO.Limit(limit))
}

func (e eventDo) Offset(offset int) IEventDo {
	return e.withDO(e.DO.Offset(offset))
}

func (e eventDo) Scopes(funcs ...func(gen.Dao) gen.Dao) IEventDo {
	return e.withDO(e.DO.Scopes(funcs...))
}

func (e eventDo) Unscoped() IEventDo {
	return e.withDO(e.DO.Unscoped())
}

func (e eventDo) Create(values ...*entity.Event) error {
	if len(values) == 0 {
		return nil
	}
	return e.DO.Create(values)
}

func (e eventDo) CreateInBatches(values []*entity.Event, batchSize int) error {
	return e.DO.CreateInBatches(values, batchSize)
}

// Save : !!! underlying implementation is different with GORM
// The method is equivalent to executing the statement: db.Clauses(clause.OnConflict{UpdateAll: true}).Create(values)
func (e eventDo) Save(values ...*entity.Event) error {
	if len(values) == 0 {
		return nil
	}
	return e.DO.Save(values)
}

func (e eventDo) First() (*entity.Event, error) {
	if result, err := e.DO.First(); err != nil {
		return nil, err
	} else {
		return result.(*entity.Event), nil
	}
}

func (e eventDo) Take() (*entity.Event, error) {
	if result, err := e.DO.Take(); err != nil {
		return nil, err
	} else {
		return result.(*entity.Event), nil
	}
}

func (e eventDo) Last() (*entity.Event, error) {
	if result, err := e.DO.Last(); err != nil {
		return nil, err
	} else {
		return result.(*entity.Event), nil
	}
}

func (e eventDo) Find() ([]*entity.Event, error) {
	result, err := e.DO.Find()
	return result.([]*entity.Event), err
}

func (e eventDo) FindInBatch(batchSize int, fc func(tx gen.Dao, batch int) error) (results []*entity.Event, err error) {
	buf := make([]*entity.Event, 0, batchSize)
	err = e.DO.FindInBatches(&buf, batchSize, func(tx gen.Dao, batch int) error {
		defer func() { results = append(results, buf...) }()
		return fc(tx, batch)
	})
	return results, err
}

func (e eventDo) FindInBatches(result *[]*entity.Event, batchSize int, fc func(tx gen.Dao, batch int) error) error {
	return e.DO.FindInBatches(result, batchSize, fc)
}

func (e eventDo) Attrs(attrs ...field.AssignExpr) IEventDo {
	return e.withDO(e.DO.Attrs(attrs...))
}

func (e eventDo) Assign(attrs ...field.AssignExpr) IEventDo {
	return e.withDO(e.DO.Assign(attrs...))
}

func (e eventDo) Joins(fields ...field.RelationField) IEventDo {
	for _, _f := range fields {
		e = *e.withDO(e.DO.Joins(_f))
	}
	return &e
}

func (e eventDo) Preload(fields ...field.RelationField) IEventDo {
	for _, _f := range fields {
		e = *e.withDO(e.DO.Preload(_f))
	}
	return &e
}

func (e eventDo) FirstOrInit() (*entity.Event, error) {
	if result, err := e.DO.FirstOrInit(); err != nil {
		return nil, err
	} else {
		return result.(*entity.Event), nil
	}
}

func (e eventDo) FirstOrCreate() (*entity.Event, error) {
	if result, err := e.DO.FirstOrCreate(); err != nil {
		return nil, err
	} else {
		return result.(*entity.Event), nil
	}
}

func (e eventDo) FindByPage(offset int, limit int) (result []*entity.Event, count int64, err error) {
	result, err = e.Offset(offset).Limit(limit).Find()
	if err != nil {
		return
	}

	if size := len(result); 0 < limit && 0 < size && size < limit {
		count = int64(size + offset)
		return
	}

	count, err = e.Offset(-1).Limit(-1).Count()
	return
}

func (e eventDo) ScanByPage(result interface{}, offset int, limit int) (count int64, err error) {
	count, err = e.Count()
	if err != nil {
		return
	}

	err = e.Offset(offset).Limit(limit).Scan(result)
	return
}

func (e eventDo) Scan(result interface{}) (err error) {
	return e.DO.Scan(result)
}

func (e eventDo) Delete(models ...*entity.Event) (result gen.ResultInfo, err error) {
	return e.DO.Delete(models)
}

func (e *eventDo) withDO(do gen.Dao) *eventDo {
	e.DO = *do.(*gen.DO)
	return e
}
