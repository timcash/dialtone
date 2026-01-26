package util

import (
	"fmt"
	"math/rand"
	"time"
)

var (
	adjectives = []string{
		"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india",
		"juliett", "kilo", "lima", "mike", "november", "oscar", "papa", "quebec", "romeo",
		"sierra", "tango", "uniform", "victor", "whiskey", "xray", "yankee", "zulu",
		"falcon", "eagle", "hawk", "raven", "condor", "viper", "cobra", "python",
		"shadow", "ghost", "phantom", "spectre", "spirit", "ranger", "scout",
	}

	nouns = []string{
		"base", "post", "outpost", "station", "command", "tower", "bunker",
		"squad", "team", "unit", "force", "group", "wing", "division",
		"point", "zone", "sector", "vector", "grid", "target",
	}
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateCodename generates a random military-style codename.
// If prefix is provided, it prepends it to the codename (e.g. "prefix-alpha-bravo").
// Otherwise it generates "adj-adj" or "adj-noun" combinations.
func GenerateCodename(prefix string) string {
	word1 := adjectives[rand.Intn(len(adjectives))]
	word2 := adjectives[rand.Intn(len(adjectives))]

	// Ensure words are different
	for word1 == word2 {
		word2 = adjectives[rand.Intn(len(adjectives))]
	}

	codename := fmt.Sprintf("%s-%s", word1, word2)

	if prefix != "" {
		return fmt.Sprintf("%s-%s", prefix, codename)
	}
	return codename
}
