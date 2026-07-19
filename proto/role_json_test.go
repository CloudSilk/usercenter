package usercenter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRoleInfoJSONIncludesDisabledState(t *testing.T) {
	data, err := json.Marshal(&RoleInfo{Enable: false})
	if err != nil {
		t.Fatalf("marshal role info: %v", err)
	}
	if !strings.Contains(string(data), `"enable":false`) {
		t.Fatalf("disabled role state must be explicit in JSON, got %s", data)
	}
}
