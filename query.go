package kallax

import (
	"fmt"

	"github.com/Masterminds/squirrel"
)

// Query returns information about some query settings and compiles the query.
type Query interface {
	compile() ([]string, squirrel.SelectBuilder)
	isReadOnly() bool
	// GetOffset returns the number of skipped rows in the query.
	GetOffset() uint64
	// GetLimit returns the max number of rows retrieved by the query.
	GetLimit() uint64
	// GetBatchSize returns the number of rows retrieved by the store per
	// batch. This is only used and has effect on queries with 1:N
	// relationships.
	GetBatchSize() uint64
}

type columnSet []string

func (cs columnSet) contains(col string) bool {
	for _, c := range cs {
		if c == col {
			return true
		}
	}
	return false
}

func (cs *columnSet) add(cols ...string) {
	for _, col := range cols {
		cs.addCol(col)
	}
}

func (cs *columnSet) addCol(col string) {
	if !cs.contains(col) {
		*cs = append(*cs, col)
	}
}

func (cs *columnSet) remove(cols ...string) {
	var newSet = make(columnSet, 0, len(*cs))
	toRemove := columnSet(cols)
	for _, col := range *cs {
		if !toRemove.contains(col) {
			newSet = append(newSet, col)
		}
	}
	*cs = newSet
}

func (cs columnSet) copy() []string {
	var result = make(columnSet, len(cs))
	for i, col := range cs {
		result[i] = col
	}
	return result
}

// BaseQuery is a generic query builder.
type BaseQuery struct {
	columns         columnSet
	excludedColumns columnSet
	builder         squirrel.SelectBuilder

	selectChanged bool
	batchSize     uint64
	offset        uint64
	limit         uint64
}

var _ Query = (*BaseQuery)(nil)

// NewBaseQuery creates a new BaseQuery for querying the given table
// and the given selected columns.
func NewBaseQuery(table string, selectedColumns ...string) *BaseQuery {
	return &BaseQuery{
		builder: squirrel.StatementBuilder.
			PlaceholderFormat(squirrel.Dollar).
			Select().
			From(table),
		columns:   columnSet(selectedColumns),
		batchSize: 50,
	}
}

func (q *BaseQuery) isReadOnly() bool {
	return q.selectChanged
}

// Select adds the given columns to the list of selected columns in the query.
func (q *BaseQuery) Select(columns ...string) {
	if !q.selectChanged {
		q.columns = columnSet{}
		q.selectChanged = true
	}

	q.excludedColumns.remove(columns...)
	q.columns.add(columns...)
}

// SelectNot adds the given columns to the list of excluded columns in the query.
func (q *BaseQuery) SelectNot(columns ...string) {
	q.excludedColumns.add(columns...)
}

// Copy returns an identical copy of the query. BaseQuery is mutable, that is
// why this method is provided.
func (q *BaseQuery) Copy() *BaseQuery {
	return &BaseQuery{
		builder:         q.builder,
		columns:         q.columns.copy(),
		excludedColumns: q.excludedColumns.copy(),
		selectChanged:   q.selectChanged,
		batchSize:       q.GetBatchSize(),
		limit:           q.GetLimit(),
		offset:          q.GetOffset(),
	}
}

func (q *BaseQuery) selectedColumns() []string {
	var result = make([]string, 0, len(q.columns))
	for _, col := range q.columns {
		if !q.excludedColumns.contains(col) {
			result = append(result, col)
		}
	}
	return result
}

// Order adds the given order clauses to the list of columns to order the
// results by.
func (q *BaseQuery) Order(cols ...ColumnOrder) {
	var c = make([]string, len(cols))
	for i, v := range cols {
		c[i] = v.ToSql()
	}
	q.builder = q.builder.OrderBy(c...)
}

// BatchSize sets the batch size.
func (q *BaseQuery) BatchSize(size uint64) {
	q.batchSize = size
}

// GetBatchSize returns the number of rows retrieved per batch while retrieving
// 1:N relationships.
func (q *BaseQuery) GetBatchSize() uint64 {
	return q.batchSize
}

// Limit sets the max number of rows to retrieve.
func (q *BaseQuery) Limit(n uint64) {
	q.limit = n
}

// GetLimit returns the max number of rows to retrieve.
func (q *BaseQuery) GetLimit() uint64 {
	return q.limit
}

// Offset sets the number of rows to skip.
func (q *BaseQuery) Offset(n uint64) {
	q.offset = n
}

// GetOffset returns the number of rows to skip.
func (q *BaseQuery) GetOffset() uint64 {
	return q.offset
}

// Where adds a new condition to filter the query. All conditions added are
// concatenated with "and".
//   q.Where(Eq(NameColumn, "foo"))
//   q.Where(Gt(AgeColumn, 18))
//   // ... WHERE name = "foo" AND age > 18
func (q *BaseQuery) Where(cond Condition) {
	q.builder = q.builder.Where(squirrel.Sqlizer(cond))
}

// compile returns the selected column names and the select builder.
func (q *BaseQuery) compile() ([]string, squirrel.SelectBuilder) {
	columns := q.selectedColumns()
	return columns, q.builder.Columns(columns...)
}

// String returns the SQL generated by the
func (q *BaseQuery) String() string {
	_, builder := q.compile()
	sql, _, _ := builder.ToSql()
	return sql
}

// ColumnOrder is a column name with its order.
type ColumnOrder interface {
	// ToSql returns the SQL representation of the column with its order.
	ToSql() string
	isColumnOrder()
}

type colOrder struct {
	order string
	col   string
}

func (o *colOrder) ToSql() string {
	return fmt.Sprintf("%s %s", o.col, o.order)
}
func (colOrder) isColumnOrder() {}

const (
	asc  = "ASC"
	desc = "DESC"
)

// Asc returns a column ordered by ascending order.
func Asc(col string) ColumnOrder {
	return &colOrder{asc, col}
}

// Desc returns a column ordered by descending order.
func Desc(col string) ColumnOrder {
	return &colOrder{desc, col}
}
