package exec

import "testing"

func TestRejectUnknownDirectiveAndTypenameArguments(t *testing.T) {
	e := newValExecutor(t)
	for _, query := range []string{`{ search(term:"ok") @skip(if:false, unknown:true) }`, `{ __typename(unknown:true) }`} {
		if errs := valErrors(t, e, query); len(errs) == 0 {
			t.Errorf("invalid query accepted: %s", query)
		}
	}
}
