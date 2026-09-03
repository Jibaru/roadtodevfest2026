package embedded

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/jibaru/agentarena/internal/battle/domain"
)

//go:embed verses.json
var versesJSON []byte

// VerseCache serves pre-written emergency verses embedded in the binary.
// If Gemini is down mid-show, the battle keeps going. Verses use the
// {topic} placeholder, filled at read time.
type VerseCache struct {
	verses map[domain.Battler][]string
}

func NewVerseCache() (*VerseCache, error) {
	var raw map[string][]string
	if err := json.Unmarshal(versesJSON, &raw); err != nil {
		return nil, fmt.Errorf("parsing embedded verses: %w", err)
	}
	c := &VerseCache{verses: map[domain.Battler][]string{}}
	for k, v := range raw {
		c.verses[domain.Battler(k)] = v
	}
	return c, nil
}

func (c *VerseCache) Emergency(battler domain.Battler, topic string) (string, error) {
	list := c.verses[battler]
	if len(list) == 0 {
		return "", domain.ErrVerseNotCached
	}
	// Deterministic pick per topic so retries return the same verse.
	h := fnv.New32a()
	h.Write([]byte(topic))
	verse := list[int(h.Sum32())%len(list)]
	return strings.ReplaceAll(verse, "{topic}", topic), nil
}
