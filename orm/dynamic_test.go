package orm

import (
	"testing"

	"github.com/stn81/dynamic"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type aDynContent struct {
	Value int `json:"value"`
}

type bDynContent struct {
	Values []int `json:"values"`
}

type dynamicModel struct {
	ID      int64         `orm:"pk;column(id)"`
	Type    string        `orm:"column(type)"`
	Content *dynamic.Type `orm:"column(content);json"`
}

func (m *dynamicModel) TableName() string {
	return "dynamic_test"
}

func (m *dynamicModel) NewDynamicField(name string) interface{} {
	switch m.Type {
	case "A":
		return new(aDynContent)
	case "B":
		return new(bDynContent)
	}
	return nil
}

func TestDynamic(t *testing.T) {
	db := NewOrm(zap.NewExample())
	_, _ = db.QueryTable(new(dynamicModel)).Delete()

	aObj := &dynamicModel{
		Type: "A",
		Content: dynamic.New(&aDynContent{
			Value: 10,
		}),
	}
	aId, err := db.Insert(aObj)
	require.NoError(t, err, "insert dyn aObj")
	t.Logf("aObj.ID=%v", aId)

	bObj := &dynamicModel{
		Type: "B",
		Content: dynamic.New(&bDynContent{
			Values: []int{1, 3, 5},
		}),
	}
	bId, err := db.Insert(bObj)
	require.NoError(t, err, "insert dyn bObj")
	t.Logf("bObj.ID=%v", bId)

	aObjRead := &dynamicModel{ID: aId}
	bObjRead := &dynamicModel{ID: bId}
	err = db.Read(aObjRead)
	require.NoError(t, err, "read dyn aObj")
	require.Equal(t, "A", aObjRead.Type, "check read dyn aObj.Type")
	require.NotNil(t, aObjRead.Content)
	require.IsType(t, aObj.Content.Value, aObjRead.Content.Value, "check read dyn aObj.Content")
	require.Equal(t, 10, aObjRead.Content.Value.(*aDynContent).Value, "check aObj.Content.Value")

	err = db.Read(bObjRead)
	require.NoError(t, err, "read dyn bObj")
	require.Equal(t, "B", bObjRead.Type, "check read dyn bObj.Type")
	require.NotNil(t, bObjRead.Content)
	require.IsType(t, bObj.Content.Value, bObjRead.Content.Value, "check read dyn bObj.Content")
	require.Equal(t, []int{1, 3, 5}, bObjRead.Content.Value.(*bDynContent).Values, "check bObj.Content.Value")
}
