package reauth

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type challenge struct {
	PrincipalID string
	Action      string
}

var (
	challenges = cache.New(5*time.Minute, 10*time.Minute)
	consumeMu  sync.Mutex
)

func Issue(principalID, action string, ttl time.Duration) (string, error) {
	proofBytes := make([]byte, 32)
	if _, err := rand.Read(proofBytes); err != nil {
		return "", err
	}
	proof := "reauth_" + hex.EncodeToString(proofBytes)
	challenges.Set(proof, challenge{
		PrincipalID: strings.TrimSpace(principalID),
		Action:      strings.TrimSpace(action),
	}, ttl)
	return proof, nil
}

func Consume(principalID, action, proof string) bool {
	proof = strings.TrimSpace(proof)
	if proof == "" {
		return false
	}
	consumeMu.Lock()
	value, ok := challenges.Get(proof)
	challenges.Delete(proof)
	consumeMu.Unlock()
	stored, valid := value.(challenge)
	return ok && valid && stored.PrincipalID == strings.TrimSpace(principalID) && stored.Action == strings.TrimSpace(action)
}
