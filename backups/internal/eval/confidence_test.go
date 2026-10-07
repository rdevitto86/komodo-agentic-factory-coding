package eval

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHedgesFindsEveryGuessAndPassesAVerdict(t *testing.T) {
	if got := Hedges("This might fix it. I think the test probably passes."); !reflect.DeepEqual(got, []string{"might", "I think", "probably"}) {
		t.Fatalf("hedges = %q", got)
	}
	if got := Hedges("Fixed: go test ./internal/gate/... exits 0; the mightiest case passes."); len(got) != 0 {
		t.Fatalf("hedges = %q; a verdict with evidence holds none", got)
	}
}

func TestActsWithoutAskingFailsOnAQuestionOrAHedge(t *testing.T) {
	cases := []struct {
		name, result, want string
	}{
		{"a verdict passes", `{"result":"DONE","summary":"go test ./... exits 0","notes":[]}`, ""},
		{"a hedge fails", `{"result":"DONE","summary":"this probably works","notes":["I think so"]}`, "hedged: probably, I think"},
		{"a question fails", `{"result":"BLOCKED","summary":"stopped","question":"which file?"}`, `asked "which file?"`},
	}
	for _, each := range cases {
		t.Run(each.name, func(t *testing.T) {
			env := newFakeEnv(t)
			env.komodo = func(context.Context, []string, ...string) Ran { return Ran{} }
			env.write(filepath.Join(".komodo", "results", "TSK-01.1.1.json"), each.result)
			err := actsWithoutAsking(context.Background(), env)
			if each.want == "" && err != nil {
				t.Fatalf("err = %v, want a pass", err)
			}
			if each.want != "" && (err == nil || !strings.Contains(err.Error(), each.want)) {
				t.Fatalf("err = %v, want %q", err, each.want)
			}
		})
	}
}
