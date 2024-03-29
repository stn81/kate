// Copyright 2018 Huan Du. All rights reserved.
// Licensed under the MIT license that can be found in the LICENSE file.

package sqlbuilder

import (
	"database/sql"
	"fmt"
)

func ExampleSelectBuilder() {
	sb := NewSelectBuilder()
	sb.Distinct().Select("id", "name", sb.As("COUNT(*)", "t"))
	sb.From("demo.user")
	sb.Where(
		sb.GreaterThan("id", 1234),
		sb.Like("name", "%Du"),
		sb.Or(
			sb.IsNull("id_card"),
			sb.In("status", 1, 2, 5),
		),
		sb.NotIn(
			"id",
			NewSelectBuilder().Select("id").From("banned"),
		), // Nested SELECT.
		"modified_at > created_at + "+sb.Var(86400), // It's allowed to write arbitrary SQL.
	)
	sb.GroupBy("status").Having(sb.NotIn("status", 4, 5))
	sb.OrderBy("modified_at").Asc()
	sb.Limit(10).Offset(5)

	stmt, args := sb.Build()
	fmt.Println(stmt)
	fmt.Println(args)

	// Output:
	// SELECT DISTINCT id, name, COUNT(*) AS t FROM demo.user WHERE id > ? AND name LIKE ? AND (id_card IS NULL OR status IN (?, ?, ?)) AND id NOT IN (SELECT id FROM banned) AND modified_at > created_at + ? GROUP BY status HAVING status NOT IN (?, ?) ORDER BY modified_at ASC LIMIT 10 OFFSET 5
	// [1234 %Du 1 2 5 86400 4 5]
}

func ExampleSelectBuilder_advancedUsage() {
	sb := NewSelectBuilder()
	innerSb := NewSelectBuilder()

	sb.Select("id", "name")
	sb.From(
		sb.BuilderAs(innerSb, "user"),
	)
	sb.Where(
		sb.In("status", Flatten([]int{1, 2, 3})...),
		sb.Between("created_at", sql.Named("start", 1234567890), sql.Named("end", 1234599999)),
	)
	sb.OrderBy("modified_at").Desc()

	innerSb.Select("*")
	innerSb.From("banned")
	innerSb.Where(
		innerSb.NotIn("name", Flatten([]string{"Huan Du", "Charmy Liu"})...),
	)

	stmt, args := sb.Build()
	fmt.Println(stmt)
	fmt.Println(args)

	// Output:
	// SELECT id, name FROM (SELECT * FROM banned WHERE name NOT IN (?, ?)) AS user WHERE status IN (?, ?, ?) AND created_at BETWEEN @start AND @end ORDER BY modified_at DESC
	// [Huan Du Charmy Liu 1 2 3 {{} start 1234567890} {{} end 1234599999}]
}

func ExampleSelectBuilder_join() {
	sb := NewSelectBuilder()
	sb.Select("u.id", "u.name", "c.type", "p.nickname")
	sb.From("user u")
	sb.Join("contract c",
		"u.id = c.user_id",
		sb.In("c.status", 1, 2, 5),
	)
	sb.JoinWithOption(RightOuterJoin, "person p",
		"u.id = p.user_id",
		sb.Like("p.surname", "%Du"),
	)
	sb.Where(
		"u.modified_at > u.created_at + " + sb.Var(86400), // It's allowed to write arbitrary SQL.
	)

	stmt, args := sb.Build()
	fmt.Println(stmt)
	fmt.Println(args)

	// Output:
	// SELECT u.id, u.name, c.type, p.nickname FROM user u JOIN contract c ON u.id = c.user_id AND c.status IN (?, ?, ?) RIGHT OUTER JOIN person p ON u.id = p.user_id AND p.surname LIKE ? WHERE u.modified_at > u.created_at + ?
	// [1 2 5 %Du 86400]
}
