package orm

import (
	"reflect"
	"testing"

	"github.com/stn81/kate/orm/sqlbuilder"
	"github.com/stretchr/testify/assert"
)

type Person struct {
	Id   int64  `orm:"pk;column(id)"`
	Name string `orm:"column(name)"`
}

func newSqlBuilderCond() *sqlbuilder.Cond {
	return &sqlbuilder.Cond{
		Args: &sqlbuilder.Args{},
	}
}

func TestCondition(t *testing.T) {
	person := &Person{}
	mi := newModelInfo(reflect.ValueOf(person))

	sql := NewCondition().And("Id", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` = $0", sql, "exact failed")

	sql = NewCondition().AndNot("Id", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "NOT `id` = $0", sql, "not exact failed")

	sql = NewCondition().And("Id__lt", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` < $0", sql, "lt failed")

	sql = NewCondition().And("Id__lte", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` <= $0", sql, "lte failed")

	sql = NewCondition().And("Id__gt", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` > $0", sql, "gt failed")

	sql = NewCondition().And("Id__gte", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` >= $0", sql, "gte failed")

	sql = NewCondition().And("Id__eq", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` = $0", sql, "eq failed")

	sql = NewCondition().And("Id__ne", 10).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` <> $0", sql, "ne failed")

	sql = NewCondition().And("Id__in", 10, 20, 30).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` IN ($0, $1, $2)", sql, "in failed")

	sql = NewCondition().And("Id__between", 10, 20).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` BETWEEN $0 AND $1", sql, "between failed")

	sql = NewCondition().And("Name__startswith", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE BINARY $0", sql, "startswith failed")

	sql = NewCondition().And("Name__istartswith", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE $0", sql, "istartswith failed")

	sql = NewCondition().And("Name__endswith", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE BINARY $0", sql, "endswith failed")

	sql = NewCondition().And("Name__iendswith", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE $0", sql, "iendswith failed")

	sql = NewCondition().And("Name__contains", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE BINARY $0", sql, "contains failed")

	sql = NewCondition().And("Name__icontains", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` LIKE $0", sql, "icontains failed")

	sql = NewCondition().And("Name__isnull", true).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` IS NULL", sql, "isnull(true) failed")

	sql = NewCondition().And("Name__isnull", false).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`name` IS NOT NULL", sql, "isnull(false) failed")

	sql = NewCondition().And("Id", 1).And("Name", "zhang").GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` = $0 AND `name` = $1", sql, "And().And() failed")

	sql = NewCondition().And("Id", 1).OrCond(NewCondition().And("Id", 10).Or("name", "zhang")).GetWhereSQL(mi, newSqlBuilderCond())
	assert.Equal(t, "`id` = $0 OR (`id` = $1 OR `name` = $2)", sql, "And().OrCond(And().Or()) failed")
}
