package model

import (
	"github.com/CloudSilk/usercenter/internal/dictionaries"
	apipb "github.com/CloudSilk/usercenter/proto"
)

// Dictionaries 字典(定义已迁至 internal/dictionaries,此处为兼容别名)。
type Dictionaries = dictionaries.Dictionaries

// 以下函数委托 internal/dictionaries(REDESIGN §4 阶段0),向后兼容。

func CreateDictionaries(m *Dictionaries) (string, error) { return dictionaries.CreateDictionaries(m) }
func UpdateDictionaries(m *Dictionaries) error           { return dictionaries.UpdateDictionaries(m) }
func DeleteDictionaries(id string) error                 { return dictionaries.DeleteDictionaries(id) }
func QueryDictionaries(req *apipb.QueryDictionariesRequest, resp *apipb.QueryDictionariesResponse, preload bool) {
	dictionaries.QueryDictionaries(req, resp, preload)
}
func GetDictionariesByID(id string) (*Dictionaries, error)       { return dictionaries.GetDictionariesByID(id) }
func GetDictionariesByIDs(ids []string) ([]*Dictionaries, error) { return dictionaries.GetDictionariesByIDs(ids) }
func GetAllDictionaries() (list []*Dictionaries, err error)      { return dictionaries.GetAllDictionaries() }
func ExportAllDictionaries(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	dictionaries.ExportAllDictionaries(req, resp)
}
func PBToDictionaries(in []*apipb.DictionariesInfo) []*Dictionaries {
	return dictionaries.PBToDictionaries(in)
}
func PBToDictionariesArray(in *apipb.DictionariesInfo) *Dictionaries {
	return dictionaries.PBToDictionariesArray(in)
}
func DictionariesArrayToPB(in []*Dictionaries) []*apipb.DictionariesInfo {
	return dictionaries.DictionariesArrayToPB(in)
}
func DictionariesToPB(in *Dictionaries) *apipb.DictionariesInfo {
	return dictionaries.DictionariesToPB(in)
}
