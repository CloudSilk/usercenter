package model

import "testing"

func TestGenPwd(t *testing.T) {
	t.Log(generatePasswd(16, PwdStrengthAdvance))
}
