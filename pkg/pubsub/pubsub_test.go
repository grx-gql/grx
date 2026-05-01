package pubsub

import "testing"

func TestFilterFuncMatches(t *testing.T) {
	f := FilterFunc(func(msg Message) bool { return msg.Topic == "ok" })
	if !f.Matches(Message{Topic: "ok"}) {
		t.Fatalf("expected match")
	}
	if f.Matches(Message{Topic: "no"}) {
		t.Fatalf("expected no match")
	}
}

func TestPayloadFunc(t *testing.T) {
	f := PayloadFunc(func(b []byte) bool { return len(b) > 2 })
	if !f.Matches(Message{Payload: []byte("abc")}) {
		t.Fatalf("expected match for len>2")
	}
	if f.Matches(Message{Payload: []byte("a")}) {
		t.Fatalf("expected no match for len<=2")
	}
}

func TestTopicMatchers(t *testing.T) {
	if !TopicEquals("a").Matches(Message{Topic: "a"}) {
		t.Fatalf("TopicEquals should match equal topic")
	}
	if TopicEquals("a").Matches(Message{Topic: "b"}) {
		t.Fatalf("TopicEquals should not match different topic")
	}
	if !TopicHasPrefix("user.").Matches(Message{Topic: "user.created"}) {
		t.Fatalf("TopicHasPrefix should match prefixed topic")
	}
	if TopicHasPrefix("user.").Matches(Message{Topic: "post.created"}) {
		t.Fatalf("TopicHasPrefix should not match unrelated topic")
	}
}

func TestAllAndAnyComposition(t *testing.T) {
	matchTopic := TopicEquals("x")
	matchPayload := PayloadFunc(func(b []byte) bool { return len(b) > 0 })

	all := All(matchTopic, matchPayload)
	if !all.Matches(Message{Topic: "x", Payload: []byte("y")}) {
		t.Fatalf("All should match when every filter matches")
	}
	if all.Matches(Message{Topic: "x"}) {
		t.Fatalf("All should fail when one filter fails")
	}
	if !All().Matches(Message{}) {
		t.Fatalf("All() with zero filters should always match")
	}

	any := Any(matchTopic, matchPayload)
	if !any.Matches(Message{Topic: "x"}) {
		t.Fatalf("Any should match when first filter matches")
	}
	if !any.Matches(Message{Topic: "z", Payload: []byte("y")}) {
		t.Fatalf("Any should match when second filter matches")
	}
	if any.Matches(Message{Topic: "z"}) {
		t.Fatalf("Any should fail when no filter matches")
	}
	if Any().Matches(Message{Topic: "z"}) {
		t.Fatalf("Any() with zero filters should never match")
	}
}

func TestAllAndAnyTolerateNilFilters(t *testing.T) {
	if !All(nil, TopicEquals("x")).Matches(Message{Topic: "x"}) {
		t.Fatalf("All should skip nil filters")
	}
	if !Any(nil, TopicEquals("x")).Matches(Message{Topic: "x"}) {
		t.Fatalf("Any should skip nil filters")
	}
}
