package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/stn81/kate/log"
	"reflect"

	"github.com/stn81/kate/orm/sqlbuilder"
	"go.uber.org/zap"
)

const (
	// ExprSep define the expression separation
	ExprSep = "__"
	// HintRouterMaster define the router hint for `force master`
	HintRouterMaster = `{"router":"m"} `
)

func (mi *modelInfo) PrepareInsert(ctx context.Context, db dbQueryer, tableSuffix string) (StmtQueryer, string, error) {
	logger := log.GetLogger(ctx).With(defaultLoggerTag)
	if mi.sharded && tableSuffix == "" {
		panic(ErrNoTableSuffix(mi.table))
	}

	table := mi.getTableBySuffix(tableSuffix)
	builder := sqlbuilder.NewInsertBuilder()

	values := make([]any, len(mi.fields.dbcols))
	for i := 0; i < len(values); i++ {
		values[i] = nil
	}

	builder.InsertInto(quote(table)).
		Cols(quoteAll(mi.fields.dbcols)...).
		Values(values...)

	query, _ := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:prepare_insert", zap.String("query", query))
	}

	stmt, err := db.PrepareContext(ctx, query)
	return stmt, query, err
}

func (mi *modelInfo) InsertStmt(ctx context.Context, stmt StmtQueryer, ind reflect.Value) (int64, error) {
	values := mi.getValues(ind, mi.fields.dbcols)
	result, err := stmt.ExecContext(ctx, values...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (mi *modelInfo) Read(ctx context.Context, db dbQueryer, ind reflect.Value, whereNames []string,
	forUpdate bool, forceMaster bool) error {
	var (
		whereColumns []string
		whereValues  []any
		table        = mi.getTableByInd(ind)
		logger       = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	if len(whereNames) > 0 {
		whereColumns = mi.getColumns(whereNames)
		whereValues = mi.getValues(ind, whereNames)
	} else {
		// default use pk value as whereNames condition.
		pkColumn, pkValue, ok := mi.getExistPk(ind)
		if !ok {
			return ErrMissPK
		}
		whereColumns = []string{pkColumn}
		whereValues = []any{pkValue}
	}

	builder := sqlbuilder.NewSelectBuilder()

	builder.Select(quoteAll(mi.fields.dbcols)...).
		From(quote(table)).
		Where(getEqualWhereExprs(&builder.Cond, quoteAll(whereColumns), whereValues)...)

	if forUpdate {
		builder.ForUpdate()
	}

	var (
		query string
		args  []any
	)

	if forceMaster {
		query, args = sqlbuilder.Build(HintRouterMaster, builder).Build()
	} else {
		query, args = builder.Build()
	}

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:read", zap.String("query", query), zap.Any("args", args))
	}

	dynColumns, containers := mi.getValueContainers(ind, mi.fields.dbcols, false)
	err := db.QueryRowContext(ctx, query, args...).Scan(containers...)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNoRows
	case err != nil:
		return err
	}

	if err = mi.setDynamicFields(ind, dynColumns); err != nil {
		return err
	}
	return nil
}

func (mi *modelInfo) Insert(ctx context.Context, db dbQueryer, ind reflect.Value) (int64, error) {
	var (
		table   = mi.getTableByInd(ind)
		values  = mi.getValues(ind, mi.fields.dbcols)
		builder = sqlbuilder.NewInsertBuilder()
		logger  = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	builder.InsertInto(quote(table)).Cols(quoteAll(mi.fields.dbcols)...).Values(values...)

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:insert", zap.String("query", query), zap.Any("args", args))
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (mi *modelInfo) Update(ctx context.Context, db dbQueryer, ind reflect.Value, setNames []string) (int64, error) {
	pkName, pkValue, ok := mi.getExistPk(ind)
	if !ok {
		return 0, ErrMissPK
	}

	var (
		setColumns []string
		logger     = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	// if specify setNames length is zero, then commit all columns.
	if len(setNames) == 0 {
		setColumns = make([]string, 0, len(mi.fields.dbcols)-1)
		for _, fi := range mi.fields.fieldsDB {
			if !fi.pk {
				setColumns = append(setColumns, fi.column)
			}
		}
	} else {
		setColumns = mi.getColumns(setNames)
	}

	if len(setColumns) == 0 {
		panic(errors.New("no columns to update"))
	}

	setValues := mi.getValues(ind, setColumns)

	table := mi.getTableByInd(ind)
	builder := sqlbuilder.NewUpdateBuilder()

	builder.Update(quote(table)).
		Set(getAssignments(builder, quoteAll(setColumns), setValues)...).
		Where(builder.E(quote(pkName), pkValue))

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:update", zap.String("query", query), zap.Any("args", args))
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (mi *modelInfo) Delete(ctx context.Context, db dbQueryer, ind reflect.Value, whereNames []string) (int64, error) {
	var (
		whereColumns []string
		whereValues  []any
		table        = mi.getTableByInd(ind)
		logger       = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	// if specify whereNames length > 0, then use it for where condition.
	if len(whereNames) > 0 {
		whereColumns = mi.getColumns(whereNames)
		whereValues = mi.getValues(ind, whereNames)
	} else {
		// default use pk value as where condition.
		pkColumn, pkValue, ok := mi.getExistPk(ind)
		if !ok {
			return 0, ErrMissPK
		}
		whereColumns = []string{pkColumn}
		whereValues = []any{pkValue}
	}

	if len(whereColumns) == 0 {
		panic(errors.New("delete no where conditions"))
	}

	builder := sqlbuilder.NewDeleteBuilder()
	builder.DeleteFrom(quote(table)).Where(getEqualWhereExprs(&builder.Cond, quoteAll(whereColumns), whereValues)...)

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:delete", zap.String("query", query), zap.Any("args", args))
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (mi *modelInfo) InsertMulti(
	ctx context.Context,
	db dbQueryer,
	sind reflect.Value,
	bulk int,
	tableSuffix string,
) (int64, error) {
	var (
		table   = mi.getTableBySuffix(tableSuffix)
		builder *sqlbuilder.InsertBuilder
		length  = sind.Len()
		count   int64
		logger  = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	if length == 0 {
		return 0, nil
	}

	bulkIdx := 0
	for i := 1; i <= length; i++ {
		if builder == nil {
			builder = sqlbuilder.NewInsertBuilder().InsertInto(quote(table)).Cols(quoteAll(mi.fields.dbcols)...)
		}

		ind := reflect.Indirect(sind.Index(i - 1))
		values := mi.getValues(ind, mi.fields.dbcols)
		builder.Values(values...)

		if i%bulk == 0 || i == length {
			bulkIdx++

			query, args := builder.Build()

			if DebugSQLBuilder {
				logger.Debug("sqlbuilder:insert_multi",
					zap.String("query", query),
					zap.Any("args", args),
					zap.Int("bulk_idx", bulkIdx))
			}

			_, err := db.ExecContext(ctx, query, args...)
			if err != nil {
				return count, err
			}
			count += int64(i % bulk)
			builder = nil
		}
	}

	return count, nil
}

func (mi *modelInfo) UpdateBatch(ctx context.Context, db dbQueryer,
	qs *querySetter, cond *Condition, params Params) (int64, error) {
	var (
		setNames  = make([]string, 0, len(params))
		setValues = make([]any, 0, len(params))
		logger    = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	for name, value := range params {
		setNames = append(setNames, name)
		setValues = append(setValues, value)
	}
	setColumns := mi.getColumns(setNames)

	table := mi.getTableBySuffix(qs.tableSuffix)
	builder := sqlbuilder.NewUpdateBuilder()

	builder.Update(quote(table)).
		Set(getAssignments(builder, quoteAll(setColumns), setValues)...)

	if cond != nil && !cond.IsEmpty() {
		builder.Where(cond.GetWhereSQL(mi, &builder.Cond))
	}

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:update_batch", zap.String("query", query), zap.Any("args", args))
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (mi *modelInfo) DeleteBatch(ctx context.Context, db dbQueryer, qs *querySetter, cond *Condition) (int64, error) {
	var (
		table   = mi.getTableBySuffix(qs.tableSuffix)
		builder = sqlbuilder.NewDeleteBuilder()
		logger  = log.GetLogger(ctx).With(defaultLoggerTag)
	)

	builder.DeleteFrom(quote(table))

	if cond != nil && !cond.IsEmpty() {
		builder.Where(cond.GetWhereSQL(mi, &builder.Cond))
	}

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:delete_batch", zap.String("query", query), zap.Any("args", args))
	}

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// nolint:gocyclo,lll
func (mi *modelInfo) getQueryArgsForRead(qs *querySetter, cond *Condition, selectNames []string) (string, []any) {
	var selectColumns []string
	if len(selectNames) > 0 {
		selectColumns = mi.getColumns(selectNames)
	} else {
		selectColumns = mi.fields.dbcols
	}

	builder := sqlbuilder.NewSelectBuilder()
	table := mi.getTableBySuffix(qs.tableSuffix)

	if qs.distinct {
		builder.Distinct()
	}

	builder.Select(quoteAll(selectColumns)...).From(quote(table))

	if cond != nil && !cond.IsEmpty() {
		builder.Where(cond.GetWhereSQL(mi, &builder.Cond))
	}

	if len(qs.orders) > 0 {
		builder.OrderBy(mi.getOrderByCols(qs.orders)...)
	}

	if len(qs.groups) > 0 {
		builder.GroupBy(mi.getGroupCols(qs.groups)...)
	}

	if qs.limit > 0 {
		builder.Limit(qs.limit)
	}

	if qs.offset > 0 {
		builder.Offset(qs.offset)
	}

	if qs.forUpdate {
		builder.ForUpdate()
	}

	var (
		query string
		args  []any
	)
	if qs.forceMaster {
		query, args = sqlbuilder.Build(HintRouterMaster, builder).Build()
	} else {
		query, args = builder.Build()
	}
	return query, args
}

// nolint:lll
func (mi *modelInfo) ReadOne(ctx context.Context, db dbQueryer, qs *querySetter, cond *Condition, container any, selectNames []string) error {
	logger := log.GetLogger(ctx).With(defaultLoggerTag)
	val := reflect.ValueOf(container)
	ind := reflect.Indirect(val)

	if val.Kind() != reflect.Pointer || mi.fullName != getFullName(ind.Type()) {
		panic(fmt.Errorf("wrong object type `%s` for rows scan, need *%s", val.Type(), mi.fullName))
	}

	if len(selectNames) == 0 {
		selectNames = mi.fields.dbcols
	}

	query, args := mi.getQueryArgsForRead(qs, cond, selectNames)

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:read_one", zap.String("query", query), zap.Any("args", args))
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	count := 0
	if rows.Next() {
		elem := reflect.New(mi.addrField.Elem().Type())
		elemInd := reflect.Indirect(elem)

		dynColumns, containers := mi.getValueContainers(elemInd, selectNames, false)
		if err = rows.Scan(containers...); err != nil {
			return err
		}

		if err = mi.setDynamicFields(elemInd, dynColumns); err != nil {
			return err
		}

		ind.Set(elemInd)
		count++
	}

	if err = rows.Err(); err != nil {
		return err
	}

	if count == 0 {
		return ErrNoRows
	}

	return nil
}

// nolint:gocyclo,lll
func (mi *modelInfo) ReadBatch(ctx context.Context, db dbQueryer, qs *querySetter, cond *Condition, container any, selectNames []string) error {
	logger := log.GetLogger(ctx).With(defaultLoggerTag)
	val := reflect.ValueOf(container)
	ind := reflect.Indirect(val)
	isPtr := true

	if val.Kind() != reflect.Pointer || ind.Kind() != reflect.Slice || ind.Len() > 0 {
		panic(fmt.Errorf("wrong object type `%s` for rows scan, need and empty slice *[]*%s or *[]%s",
			val.Type(),
			mi.fullName,
			mi.fullName))
	}

	fn := ""
	typ := ind.Type().Elem()
	switch typ.Kind() {
	case reflect.Pointer:
		fn = getFullName(typ.Elem())
	case reflect.Struct:
		isPtr = false
		fn = getFullName(typ)
	}

	if mi.fullName != fn {
		panic(fmt.Errorf("wrong object type `%s` for rows scan, need *[]*%s or *[]%s",
			val.Type(),
			mi.fullName,
			mi.fullName))
	}

	if len(selectNames) == 0 {
		selectNames = mi.fields.dbcols
	}

	query, args := mi.getQueryArgsForRead(qs, cond, selectNames)

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:read_batch", zap.String("query", query), zap.Any("args", args))
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	slice := reflect.MakeSlice(ind.Type(), 0, 0)
	for rows.Next() {
		elem := reflect.New(mi.addrField.Elem().Type())
		elemInd := reflect.Indirect(elem)

		dynColumns, containers := mi.getValueContainers(elemInd, selectNames, false)
		if err = rows.Scan(containers...); err != nil {
			return err
		}

		if err = mi.setDynamicFields(elemInd, dynColumns); err != nil {
			return err
		}

		if isPtr {
			slice = reflect.Append(slice, elemInd.Addr())
		} else {
			slice = reflect.Append(slice, elemInd)
		}
	}

	if err = rows.Err(); err != nil {
		return err
	}

	ind.Set(slice)

	return nil
}

// nolint:lll
func (mi *modelInfo) Count(ctx context.Context, db dbQueryer, qs *querySetter, cond *Condition) (count int64, err error) {
	logger := log.GetLogger(ctx).With(defaultLoggerTag)
	table := mi.getTableBySuffix(qs.tableSuffix)
	builder := sqlbuilder.NewSelectBuilder()
	builder.Select("COUNT(1)").From(quote(table))

	if cond != nil && !cond.IsEmpty() {
		builder.Where(cond.GetWhereSQL(mi, &builder.Cond))
	}

	query, args := builder.Build()

	if DebugSQLBuilder {
		logger.Debug("sqlbuilder:count", zap.String("query", query), zap.Any("args", args))
	}

	err = db.QueryRowContext(ctx, query, args...).Scan(&count)
	return
}

// getEqualWhereExprs return where exprs used in sqlbuilder.Cond
func getEqualWhereExprs(cond *sqlbuilder.Cond, columns []string, values []any) []string {
	whereExprs := make([]string, len(columns))
	for i := range columns {
		whereExprs[i] = cond.E(columns[i], values[i])
	}
	return whereExprs
}

// getAssignments return set exprs used in sqlbuilder.UpdateBuilder
func getAssignments(ub *sqlbuilder.UpdateBuilder, columns []string, values []any) []string {
	assignments := make([]string, len(columns))
	for i := range columns {
		switch v := values[i].(type) {
		case *colValue:
			switch v.op {
			case ColAdd:
				assignments[i] = ub.Add(columns[i], v.value)
			case ColSub:
				assignments[i] = ub.Sub(columns[i], v.value)
			case ColMul:
				assignments[i] = ub.Mul(columns[i], v.value)
			case ColDiv:
				assignments[i] = ub.Div(columns[i], v.value)
			}
		default:
			assignments[i] = ub.Assign(columns[i], v)
		}
	}
	return assignments
}
