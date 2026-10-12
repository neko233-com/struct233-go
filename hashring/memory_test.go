package hashring

import "testing"

func TestRemoveClearsUnusedOwnerBuffer(t *testing.T) {
	r := New(WithReplicas(4))
	r.Add("alpha", "beta", "gamma")
	for _, node := range []string{"alpha", "beta", "gamma"} {
		r.Remove(node)
		for _, owner := range r.owners[len(r.owners):cap(r.owners)] {
			if owner != "" {
				t.Fatal("Remove retained a node name outside the active owner slice")
			}
		}
	}
}
