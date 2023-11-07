package orm

import (
	"testing"

	"github.com/stn81/dynamic"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type aData struct {
	Value int `json:"value"`
}

type bData struct {
	Values []int `json:"items"`
}

type jsonContent struct {
	Type string        `json:"type"`
	Data *dynamic.Type `json:"data"`
}

func (c *jsonContent) NewDynamicField(name string) any {
	switch c.Type {
	case "a":
		return new(aData)
	case "b":
		return new(bData)
	}
	return nil
}

type jsonModel struct {
	Id         int64        `json:"id" orm:"pk;column(id)"`
	Content    jsonContent  `json:"content" orm:"column(content);json"`
	ContentPtr *jsonContent `json:"content_ptr" orm:"column(content_ptr);json"`
}

func (*jsonModel) TableName() string {
	return "json_test"
}

func TestJSON(t *testing.T) {
	//aRawContent := `{"type": "a", "content": {"value": 10}}`
	//bRawContent := `{"type": "b", "content": {"items": [1,3,5]}}`

	db := NewOrm(zap.NewExample())
	_, _ = db.QueryTable(new(jsonModel)).Delete()

	aObj := &jsonModel{
		Content: jsonContent{
			Type: "a",
			Data: dynamic.New(&aData{Value: 10}),
		},
		ContentPtr: &jsonContent{
			Type: "a",
			Data: dynamic.New(&aData{Value: 10}),
		},
	}

	aId, err := db.Insert(aObj)
	require.NoError(t, err, "insert aObj")
	t.Logf("aObj.Id=%v", aId)

	bObj := &jsonModel{
		Content: jsonContent{
			Type: "b",
			Data: dynamic.New(&bData{Values: []int{1, 3, 5}}),
		},
		ContentPtr: &jsonContent{
			Type: "b",
			Data: dynamic.New(&bData{Values: []int{1, 3, 5}}),
		},
	}

	bId, err := db.Insert(bObj)
	require.NoError(t, err, "insert bObj")
	t.Logf("bObj.Id=%v", bId)

	aObjRead := &jsonModel{Id: aId}
	bObjRead := &jsonModel{Id: bId}

	err = db.Read(aObjRead)
	require.NoError(t, err, "read aObj")
	require.IsType(t, aObj.Content, aObjRead.Content)
	require.Equal(t, aObj.Content.Type, aObjRead.Content.Type)
	adata := aObjRead.Content.Data.Value.(*aData)
	require.Equal(t, 10, adata.Value, "check aObjRead.Content.Value")
	adataFromPtr := aObjRead.ContentPtr.Data.Value.(*aData)
	require.Equal(t, 10, adataFromPtr.Value, "check aObjRead.ContentPtr.Value")

	err = db.Read(bObjRead)
	require.NoError(t, err, "read bObj")
	require.IsType(t, bObj.Content, bObjRead.Content)
	require.Equal(t, bObj.Content.Type, bObjRead.Content.Type)
	bdata := bObjRead.Content.Data.Value.(*bData)
	require.Equal(t, []int{1, 3, 5}, bdata.Values, "check aObjRead.Content.Values")
	bdataFromPtr := bObjRead.ContentPtr.Data.Value.(*bData)
	require.Equal(t, []int{1, 3, 5}, bdataFromPtr.Values, "check aObjRead.ContentPtr.Values")
}

type mapJsonModel struct {
	Id      int64          `json:"id" orm:"pk;column(id)"`
	Content map[string]any `json:"content" orm:"column(content);json"`
}

func (*mapJsonModel) TableName() string {
	return "json_test2"
}

func TestJSONMap(t *testing.T) {
	db := NewOrm(zap.NewExample())
	_, _ = db.QueryTable(new(mapJsonModel)).Delete()

	content := map[string]any{
		"zhangsan": "1",
		"lisi":     "200",
	}
	mapObj := &mapJsonModel{Content: content}
	id, err := db.Insert(mapObj)
	require.NoError(t, err, "insert map json obj")

	mapObjRead := &mapJsonModel{Id: id}
	err = db.Read(mapObjRead)
	require.NoError(t, err, "read map json obj")
	require.Equal(t, content, mapObjRead.Content, "check map json content readed")
}
