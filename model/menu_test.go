package model

import (
	"encoding/json"
	"testing"
)

func TestAddMenu(t *testing.T) {
	t.Log(AddMenu(&Menu{
		Level:     1,
		ParentID:  "100000000",
		Path:      "dashboard",
		Name:      "dashboard",
		Hidden:    false,
		Component: "view/dashboard/index.vue",
		Sort:      1,
		Title:     "仪表盘", Icon: "setting",
		MenuFuncs: []*MenuFunc{
			{Name: "Test", Title: "测试", Hidden: false,
				MenuFuncApis: []MenuFuncApi{
					{APIID: "1"},
				}},
		},
	}))
}

func TestGetMenuByID(t *testing.T) {
	menu, err := GetMenuByID("108010000")
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := json.Marshal(menu)
	t.Logf("%s", buf)
}

func TestGetBaseMenuTree(t *testing.T) {
	menu, err := GetBaseMenuTree()
	if err != nil {
		t.Fatal(err)
	}
	buf, _ := json.Marshal(menu)
	t.Logf("%s", buf)
}
