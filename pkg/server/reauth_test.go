package server

import (
	"testing"
	"time"

	"github.com/CloudSilk/usercenter/internal/reauth"
)

func TestConsumeReauthProof(t *testing.T) {
	proof, err := reauth.Issue("user-1", "ai_execute:suggestion-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !ConsumeReauthProof("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("valid proof rejected")
	}
	if ConsumeReauthProof("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("proof accepted more than once")
	}
}
