package orm

import (
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type obj struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type anyObj struct {
	Id      int64 `orm:"column(id);pk"`
	ObjOmit obj   `orm:"column(obj_omit);json(omitempty)"`
	Obj     obj   `orm:"column(obj);json"`
}

func (o *anyObj) TableName() string {
	return "any_obj"
}

type shardedPerson struct {
	Id        int64  `orm:"column(id);pk;auto"`
	PersonId  int64  `orm:"column(person_id)"`
	Name      string `orm:"column(name)"`
	Age       int    `orm:"column(age)"`
	BirthTime string `orm:"-"`
}

func (*shardedPerson) TableName() string {
	return "person"
}

func (p *shardedPerson) TableSuffix() string {
	return strconv.FormatInt(p.PersonId%4, 10)
}

func TestInsert(t *testing.T) {
	for i := 0; i < 4; i++ {
		tableSuffix := strconv.Itoa(i)
		delCnt, err := NewOrm(zap.NewExample()).QueryTable(new(shardedPerson)).WithSuffix(tableSuffix).Delete()
		require.NoError(t, err, "delete all rows failed")
		t.Logf("before TestInsert, rows deleted: count=%v", delCnt)
	}

	db := NewOrm(zap.NewExample())
	person1 := &shardedPerson{
		PersonId: 1,
		Name:     "zhang",
		Age:      18,
	}

	id, err := db.Insert(person1)
	require.NoError(t, err, "insert person1 failed")
	require.Equal(t, id, person1.Id, "check person1.Id")

	person2 := &shardedPerson{
		PersonId: 2,
		Name:     "lisi",
		Age:      20,
	}
	id, err = db.Insert(person2)
	require.NoError(t, err, "insert person2 failed")
	require.Equal(t, id, person2.Id, "check person2.Id")

	person3 := &shardedPerson{
		PersonId: 3,
		Name:     "wang",
		Age:      20,
	}
	id, err = db.Insert(person3)
	require.NoError(t, err, "insert person3 failed")
	require.Equal(t, id, person3.Id, "check person3.Id")

	persons := []*shardedPerson{
		{PersonId: 3, Name: "multi_3"},
		{PersonId: 7, Name: "multi_7"},
		{PersonId: 11, Name: "multi_11"},
	}

	_, err = db.InsertMulti(10, persons)
	require.NoError(t, err, "insertMulti persons failed")

	for i := 0; i < 4; i++ {
		tableSuffix := strconv.Itoa(i)
		delCnt, err := NewOrm(zap.NewExample()).QueryTable(new(shardedPerson)).WithSuffix(tableSuffix).Delete()
		require.NoError(t, err, "delete all rows failed")
		t.Logf("after TestInsert, rows deleted: count=%v", delCnt)
	}
}

func TestRead(t *testing.T) {
	personAdd := &shardedPerson{
		PersonId: 21,
		Name:     "zhangsan",
		Age:      30,
	}

	// insert person
	db := NewOrm(zap.NewExample())
	id, err := db.Insert(personAdd)
	require.NoError(t, err, "insert person failed")

	// read by pk
	personReadByPk := &shardedPerson{
		Id:       id,
		PersonId: 1, // fake person id for sharding
	}
	err = db.Read(personReadByPk)
	require.NoError(t, err, "read person by pk failed")
	require.Equal(t, personAdd.PersonId, personReadByPk.PersonId, "person read by pk: check PersonId ")
	require.Equal(t, personAdd.Name, personReadByPk.Name, "person read by pk: check Name ")

	// read by cols
	personReadByCols := &shardedPerson{
		PersonId: 21,
		Name:     "zhangsan",
	}
	err = db.Read(personReadByCols, "PersonId", "Name")
	require.NoError(t, err, "read person by cols failed")
	require.Equal(t, personAdd.Id, personReadByCols.Id, "person read by cols: check pk Id")

	// delete person
	num, err := db.Delete(personAdd)
	require.NoError(t, err, "delete person added failed")
	require.Equal(t, int64(1), num, "delete person added: check num deleted =1")
}

func TestUpdate(t *testing.T) {
	personAdd := &shardedPerson{
		PersonId: 21,
		Name:     "zhangsan",
		Age:      30,
	}

	// insert person
	db := NewOrm(zap.NewExample())
	id, err := db.Insert(personAdd)
	require.NoError(t, err, "insert person failed")
	require.Equal(t, id, personAdd.Id)

	// update person
	personUpdated := &shardedPerson{
		Id:       personAdd.Id,
		PersonId: 1, // fake sharding id
		Name:     "lisi",
	}
	rowsAffected, err := db.Update(personUpdated, "Name")
	require.NoError(t, err, "update person failed")
	require.Equal(t, int64(1), rowsAffected, "rows affected != 1")

	// reload person
	personReloaded := &shardedPerson{
		Id:       personAdd.Id,
		PersonId: 1, // fake sharding id
	}
	err = db.Read(personReloaded)
	require.NoError(t, err, "reload person failed")
	require.Equal(t, personUpdated.Name, personReloaded.Name)
	require.Equal(t, personAdd.PersonId, personReloaded.PersonId)
	require.Equal(t, personAdd.Age, personReloaded.Age)

	// delete it
	num, err := db.Delete(personAdd)
	require.NoError(t, err, "delete person added failed")
	require.Equal(t, int64(1), num, "delete person added: check num deleted =1")
}

func TestQueryTable(t *testing.T) {
	db := NewOrm(zap.NewExample())
	_, err := db.QueryTable(new(shardedPerson)).WithSuffix("0").Delete()
	require.NoError(t, err, "clean person table")

	person1 := &shardedPerson{
		PersonId: 20,
		Name:     "zhangsan",
		Age:      10,
	}
	person2 := &shardedPerson{
		PersonId: 40,
		Name:     "lisi",
		Age:      9,
	}
	db.Insert(person1)
	db.Insert(person2)

	// Count
	qs := db.QueryTable(new(shardedPerson)).WithSuffix("0").Filter("Age__gte", 9)
	count, err := qs.Count()
	require.NoError(t, err, "query count failed")
	require.Equal(t, int64(2), count, "count != 2")

	// ReadBatch
	var persons []*shardedPerson
	err = qs.OrderBy("-Age").All(&persons)
	require.NoError(t, err, "query all failed")

	require.Equal(t, 2, len(persons), "len(persons) != 2")
	require.Equal(t, person1.PersonId, persons[0].PersonId, "check person[0].PersonId")
	require.Equal(t, person1.Name, persons[0].Name, "check person[0].Name")
	require.Equal(t, person1.Age, persons[0].Age, "check person[0].Age")
	require.Equal(t, person2.PersonId, persons[1].PersonId, "check person[1].PersonId")
	require.Equal(t, person2.Name, persons[1].Name, "check person[1].Name")
	require.Equal(t, person2.Age, persons[1].Age, "check person[1].Age")

	// ReadOne
	personOne := shardedPerson{}
	err = qs.OrderBy("-Age").One(&personOne)
	require.NoError(t, err, "query one failed")
	require.Equal(t, person1.PersonId, personOne.PersonId, "check personOne.PersonId")
	require.Equal(t, person1.Name, personOne.Name, "check personOne.Name")
	require.Equal(t, person1.Age, personOne.Age, "check personOne.Age")

	personOneAsc := shardedPerson{}
	err = qs.OrderBy("Age").One(&personOneAsc)
	require.NoError(t, err, "query one asc failed")
	require.Equal(t, person2.PersonId, personOneAsc.PersonId, "check personOneAsc.PersonId")
	require.Equal(t, person2.Name, personOneAsc.Name, "check personOneAsc.Name")
	require.Equal(t, person2.Age, personOneAsc.Age, "check personOneAsc.Age")

	// UpateBatch
	rowsAffected, err := qs.Update(Params{"Age": 18})
	require.NoError(t, err, "update set age = 18 failed")
	require.Equal(t, int64(2), rowsAffected, "rowsAffected != 2")
	// UpdateBatch with column op
	rowsAffected, err = qs.Filter("Age", 18).Update(Params{"Age": ColValue(ColSub, 1)})
	require.NoError(t, err, "update set age = age -1 failed")
	require.Equal(t, int64(2), rowsAffected, "rowsAffected != 2")

	// DeleteBatch
	var personsUpdated []shardedPerson
	err = db.QueryTable(new(shardedPerson)).WithSuffix("0").All(&personsUpdated)
	require.NoError(t, err, "query all after update")
	require.Equal(t, 17, personsUpdated[0].Age, "check personsUpdated[0].Age")
	require.Equal(t, 17, personsUpdated[1].Age, "check personsUpdated[1].Age")

	rowsDeleted, err := db.QueryTable(new(shardedPerson)).WithSuffix("0").Filter("PersonId__in", 20, 40).Delete()
	require.NoError(t, err, "detele for person_id in (20,40)")
	require.Equal(t, int64(2), rowsDeleted, "rowsDeleted != 2")

	rowsDeleted, err = db.QueryTable(new(shardedPerson)).WithSuffix("0").Filter("PersonId__in", []int64{20, 40}).Delete()
	require.NoError(t, err, "2nd detele for person_id in (20,40)")
	require.Equal(t, int64(0), rowsDeleted, "rowsDeleted != 0")
}

func TestJsonOmit(t *testing.T) {
	db := NewOrm(zap.NewExample())
	db.QueryTable(new(anyObj)).Delete()

	obj := &anyObj{
		Id:      1,
		ObjOmit: obj{"", 0},
		Obj:     obj{"", 0},
	}
	_, err := db.Insert(obj)
	require.NoError(t, err)
}

func TestMultiDBSameTable(t *testing.T) {
	db := NewOrm(zap.NewExample())
	o1 := &anyObj{
		Id:      1,
		ObjOmit: obj{"", 1},
		Obj:     obj{"", 1},
	}

	db.Using("default")
	db.QueryTable(new(anyObj)).Delete()
	_, err := db.Insert(o1)
	require.NoError(t, err)

	o2 := &anyObj{
		Id:      2,
		ObjOmit: obj{"", 2},
		Obj:     obj{"", 2},
	}
	db.Using("orm_test2")
	db.QueryTable(new(anyObj)).Delete()
	_, err = db.Insert(o2)
	require.NoError(t, err)

	db.Using("default")
	objRead1DB1 := &anyObj{Id: 1}
	err = db.Read(objRead1DB1, "Id")
	require.NoError(t, err)
	objRead2DB1 := &anyObj{Id: 2}
	err = db.Read(objRead2DB1, "Id")
	require.Equal(t, err, ErrNoRows)

	db.Using("orm_test2")
	objRead2DB2 := &anyObj{Id: 2}
	err = db.Read(objRead2DB2, "Id")
	require.NoError(t, err)
	objRead1DB2 := &anyObj{Id: 1}
	err = db.Read(objRead1DB2, "Id")
	require.Equal(t, err, ErrNoRows)
}

func TestRawQueryRows(t *testing.T) {
	db := NewOrm(zap.NewExample())
	db.QueryTable(new(anyObj)).Delete()

	obj := &anyObj{
		Id:  10,
		Obj: obj{"helloworld", 100},
	}
	_, err := db.Insert(obj)
	require.NoError(t, err)

	var objs []anyObj
	err = db.Raw("select obj from any_obj where id = ?", 10).QueryRows(&objs)
	require.NoError(t, err)
	require.Equal(t, 1, len(objs))
	require.Equal(t, "helloworld", objs[0].Obj.Name)
	require.Equal(t, 100, objs[0].Obj.Value)
}

type timeObj struct {
	Id      int       `orm:"column(id);pk;auto"`
	ObjTime time.Time `orm:"column(obj_time)"`
}

func (*timeObj) TableName() string {
	return "time_obj"
}

func TestTime(t *testing.T) {
	db := NewOrm(zap.NewExample())
	obj := &timeObj{
		ObjTime: time.Now(),
	}
	_, err := db.Insert(obj)
	require.NoError(t, err, "insert time obj")
	obj2 := &timeObj{
		Id: obj.Id,
	}
	err = db.Read(obj2, "Id")
	require.NoError(t, err, "read time obj")
}

func TestMain(m *testing.M) {
	RegisterDB("default", "mysql", "orm_test:orm_test@tcp(127.0.0.1:3306)/orm_test?timeout=5s&readTimeout=15s&writeTimeout=15s&parseTime=true", 20, 100)
	RegisterDB("orm_test2", "mysql", "orm_test:orm_test@tcp(127.0.0.1:3306)/orm_test2?timeout=5s&readTimeout=15s&writeTimeout=15s", 20, 100)
	RegisterModel("default", new(shardedPerson))
	RegisterModel("default", new(jsonModel))
	RegisterModel("default", new(mapJsonModel))
	RegisterModel("default", new(dynamicModel))
	RegisterModel("default", new(anyObj))
	RegisterModel("default", new(timeObj))
	DebugSQLBuilder = true
	devLogger, _ := zap.NewDevelopment()
	SetDefaultLogger(devLogger)
	os.Exit(m.Run())
}
