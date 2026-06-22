package model

import (
	"github.com/CloudSilk/usercenter/internal/language"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type Language = language.Language

func CreateLanguage(m *Language) (string, error)   { return language.CreateLanguage(m) }
func UpdateLanguage(m *Language) error              { return language.UpdateLanguage(m) }
func DeleteLanguage(id string) error                { return language.DeleteLanguage(id) }
func QueryLanguage(req *apipb.QueryLanguageRequest, resp *apipb.QueryLanguageResponse, preload bool) {
	language.QueryLanguage(req, resp, preload)
}
func GetLanguageByID(id string) (*Language, error)       { return language.GetLanguageByID(id) }
func GetLanguageByIDs(ids []string) ([]*Language, error) { return language.GetLanguageByIDs(ids) }
func GetAllLanguages() (list []*Language, err error)     { return language.GetAllLanguages() }
func ExportAllLanguages(req *apipb.CommonExportRequest, resp *apipb.CommonExportResponse) {
	language.ExportAllLanguages(req, resp)
}
func PBToLanguages(in []*apipb.LanguageInfo) []*Language { return language.PBToLanguages(in) }
func PBToLanguage(in *apipb.LanguageInfo) *Language       { return language.PBToLanguage(in) }
func LanguagesToPB(in []*Language) []*apipb.LanguageInfo  { return language.LanguagesToPB(in) }
func LanguageToPB(in *Language) *apipb.LanguageInfo       { return language.LanguageToPB(in) }
