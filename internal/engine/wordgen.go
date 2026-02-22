package engine

import (
	"fmt"
	"math/rand"
)

var adjectives = []string{
	"crimson", "rusty", "silent", "velvet", "cosmic", "lazy", "frosty", "hollow",
	"neon", "dusty", "bitter", "copper", "phantom", "savage", "orbital", "molten",
	"rogue", "solar", "wicked", "amber", "marble", "shadow", "frozen", "broken",
	"mystic", "golden", "iron", "crystal", "thunder", "pixel", "turbo", "lunar",
	"scarlet", "toxic", "atomic", "chrome", "primal", "void", "rapid", "stealth",
}

var nouns = []string{
	"wall", "river", "falcon", "lantern", "cipher", "forge", "anchor", "compass",
	"echo", "comet", "beacon", "blade", "nexus", "prism", "storm", "orbit",
	"pulse", "spectre", "vertex", "zenith", "spark", "ridge", "vortex", "helix",
	"shard", "raven", "thorn", "ghost", "ember", "lotus", "summit", "frost",
	"flux", "nova", "quasar", "arrow", "wraith", "mesa", "spire", "dusk",
}

func GenerateName() string {
	a := adjectives[rand.Intn(len(adjectives))]
	n := nouns[rand.Intn(len(nouns))]
	return fmt.Sprintf("%s-%s", a, n)
}

func GenerateEmail(name string) string {
	return name + "@mail.com"
}
