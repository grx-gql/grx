package exec_test

import (
	"context"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
)

func roadmapSubscriptionSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := exec.ParseSDL(`type Query { ping: String } type Subscription { event(limit: Int = 7): String }`)
	if err != nil {
		t.Fatal(err)
	}
	s.Subscription.Fields["event"].Resolver = func(context.Context, schema.ResolveParams) (any, error) {
		stream := make(chan string)
		close(stream)
		return stream, nil
	}
	return s
}

func TestRoadmapAuditSubscriptionCollectedRoot(t *testing.T) {
	for _, query := range []string{
		`subscription { ... on Subscription { event } }`,
		`subscription { ...Root } fragment Root on Subscription { event }`,
		`subscription { event event }`,
	} {
		t.Run(query, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := exec.New(roadmapSubscriptionSchema(t), nil).Subscribe(ctx, core.Request{Query: query}); err != nil {
				t.Fatalf("valid subscription with one collected root field rejected: %v", err)
			}
		})
	}
}

func TestSubscriptionRejectsInvalidCollectedRoots(t *testing.T) {
	for _, query := range []string{
		`subscription {}`,
		`subscription Named { first: event second: event }`,
		`subscription { __typename }`,
		`subscription { event @skip(if: false) }`,
		`subscription { ...Root @include(if: true) } fragment Root on Subscription { event }`,
		`subscription { ...Loop } fragment Loop on Subscription { ...Loop event }`,
		`subscription { ...Missing }`,
	} {
		t.Run(query, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := exec.New(roadmapSubscriptionSchema(t), nil).Subscribe(ctx, core.Request{Query: query}); err == nil {
				t.Fatal("invalid subscription accepted")
			}
		})
	}
}

func TestSubscriptionRepeatedRootFragmentIsCollectedOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	query := `subscription { ...Root ...Root } fragment Root on Subscription { event }`
	if _, err := exec.New(roadmapSubscriptionSchema(t), nil).Subscribe(ctx, core.Request{Query: query}); err != nil {
		t.Fatal(err)
	}
}
