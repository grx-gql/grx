package http

import (
	"encoding/json"
	"math"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/grx-gql/grx/core"
)

type nilResponseMarshaler struct{}

func (*nilResponseMarshaler) MarshalJSON() ([]byte, error) { panic("nil marshaler must not be called") }

func TestOrdinaryResponseEncodingCompatibility(t *testing.T) {
	ordered := core.NewOrderedObject(3)
	ordered.Set("escaped", "<>&\u2028\u2029")
	ordered.Set("custom", json.RawMessage(` {"html":"<>&"} `))
	ordered.Set("nil", (*nilResponseMarshaler)(nil))
	invalid := core.NewOrderedObject(1)
	invalid.Set("custom", json.RawMessage(`invalid`))
	transport := New(Config{})
	for _, tc := range []struct {
		name     string
		response core.Response
	}{
		{"ordered", core.Response{Data: ordered}},
		{"typed nil ordered", core.Response{Data: (*core.OrderedObject)(nil)}},
		{"null", core.Response{DataNull: true}},
		{"omitted", core.Response{}},
		{"nil custom", core.Response{Data: (*nilResponseMarshaler)(nil)}},
		{"nil list", core.Response{Data: []any(nil)}},
		{"generic data", core.Response{Data: map[string]any{"escaped": "<>&\u2028\u2029"}}},
		{"partial", core.Response{Data: ordered, Errors: []core.Error{{Message: "<>&\u2028"}}, Extensions: map[string]any{"trace": true}}},
		{"invalid custom", core.Response{Data: invalid}},
		{"nonfinite", core.Response{Data: math.NaN()}},
		{"unsupported", core.Response{Data: func() {}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expected, err := json.Marshal(tc.response)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(nethttp.MethodGet, "/graphql", nil)
			transport.writeGraphQLPayload(recorder, request, nethttp.StatusCreated, core.MediaTypeJSON, tc.response)
			if err != nil {
				if recorder.Code != nethttp.StatusInternalServerError || recorder.Body.String() != err.Error()+"\n" {
					t.Fatalf("status=%d body=%q expected error=%q", recorder.Code, recorder.Body.String(), err.Error())
				}
				if recorder.Header().Get("Content-Type") != "text/plain; charset=utf-8" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("error headers=%v", recorder.Header())
				}
				return
			}
			if recorder.Code != nethttp.StatusCreated || recorder.Body.String() != string(expected) {
				t.Fatalf("status=%d body=%s expected=%s", recorder.Code, recorder.Body.String(), expected)
			}
			if recorder.Header().Get("Content-Type") != core.MediaTypeJSON+"; charset=utf-8" || recorder.Header().Get("Content-Encoding") != "" {
				t.Fatalf("headers=%v", recorder.Header())
			}
		})
	}
}
