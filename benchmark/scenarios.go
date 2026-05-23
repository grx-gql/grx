package benchmark

// benchmarkScenario describes one comparative benchmark: a shape production
// clients often send — named operations, variables, fragments — while still
// using in-memory fixtures so library overhead dominates I/O cost.
//
// operationName must be non-empty whenever the GraphQL document contains a
// named operation (persisted-query style).
type benchmarkScenario struct {
	name          string
	query         string
	operationName string
	variables     map[string]any
}

// Scenario catalog (see README).

var (
	scenarioPersistedCompound = benchmarkScenario{
		name: "PersistedCompound",
		query: `
fragment UserAttrs on User {
  id
  name
  email
}

query PersistedOrgHome($staffCount: Int!, $heroPost: String!) {
  me: user(id: "user_1") {
    ...UserAttrs
  }
  roster: users(count: $staffCount) {
    ...UserAttrs
  }
  highlight: post(id: $heroPost) {
    id
    title
    author {
      ...UserAttrs
    }
  }
}`,
		operationName: "PersistedOrgHome",
		variables: map[string]any{
			"staffCount": 24,
			"heroPost":   "post_5",
		},
	}

	scenarioParameterizedNested = benchmarkScenario{
		name: "ParameterizedNested",
		query: `
query DetailScreen($pid: String!) {
  asset: post(id: $pid) {
    id
    title
    body
    author {
      id
      name
      email
    }
  }
}`,
		operationName: "DetailScreen",
		variables: map[string]any{
			"pid": "post_1",
		},
	}

	scenarioFeedTimeline = benchmarkScenario{
		name: "FeedTimeline",
		query: `
query HomeFeed($take: Int!) {
  timeline: feed(limit: $take) {
    id
    title
    author {
      id
      name
      email
    }
  }
}`,
		operationName: "HomeFeed",
		variables: map[string]any{
			"take": 20,
		},
	}
)

var scenarioCatalog = []benchmarkScenario{
	scenarioPersistedCompound,
	scenarioParameterizedNested,
	scenarioFeedTimeline,
}

func shallowCopyVars(vars map[string]any) map[string]any {
	if vars == nil {
		return nil
	}
	out := make(map[string]any, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}
