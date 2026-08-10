package reauth

import (
	"testing"
	"time"
)

func TestConsumeRequiresExactPrincipalAndActionAndIsSingleUse(t *testing.T) {
	proof, err := Issue("user-1", "ai_execute:suggestion-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if Consume("user-2", "ai_execute:suggestion-1", proof) {
		t.Fatal("proof accepted for another principal")
	}
	if Consume("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("failed principal check did not consume proof")
	}

	proof, err = Issue("user-1", "ai_execute:suggestion-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if Consume("user-1", "ai_execute:suggestion-2", proof) {
		t.Fatal("proof accepted for another action")
	}
	if Consume("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("failed action check did not consume proof")
	}

	proof, err = Issue("user-1", "ai_execute:suggestion-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !Consume("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("valid proof rejected")
	}
	if Consume("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("proof accepted more than once")
	}
}

func TestConsumeRejectsExpiredProof(t *testing.T) {
	proof, err := Issue("user-1", "ai_execute:suggestion-1", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if Consume("user-1", "ai_execute:suggestion-1", proof) {
		t.Fatal("expired proof accepted")
	}
}
