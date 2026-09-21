package targets

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRepoLists checks the lists committed in lists/.
func TestRepoLists(t *testing.T) {
	dir := filepath.Join("..", "..", "lists")
	ts, err := LoadSectors(filepath.Join(dir, "sectors"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]map[string]bool{}
	for _, tg := range ts {
		s := tg.Sectors[0]
		if seen[s] == nil {
			seen[s] = map[string]bool{}
		}
		if seen[s][tg.Domain] {
			t.Errorf("%s: duplicate domain %s", s, tg.Domain)
		}
		seen[s][tg.Domain] = true
	}
	for _, s := range []string{"banks", "government", "hospitals", "webshops"} {
		if len(seen[s]) < 10 {
			t.Errorf("%s: only %d domains", s, len(seen[s]))
		}
	}
	f, err := os.Open(filepath.Join(dir, "exclude.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := ParseExclude(f); err != nil {
		t.Fatal(err)
	}
}
