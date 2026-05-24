package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/auth/graph"
	"github.com/grx-gql/grx/examples/auth/session"
)

// demoBearerToken is deliberately static so you can paste it into GraphiQL's
// HTTP headers pane.
//
//	In GraphiQL: open the "Headers" tab and paste
//	  { "Authorization": "Bearer demo-alice-token" }
const demoBearerToken = "demo-alice-token"

func parseBearer(header string) (token string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, hasBearerScheme := parseBearer(r.Header.Get("Authorization"))
		if hasBearerScheme {
			switch token {
			case demoBearerToken:
				ctx = session.ContextWithSubject(ctx, "alice")
			case "":
				http.Error(w, `invalid Authorization header`, http.StatusBadRequest)
				return
			default:
				http.Error(w, `invalid bearer token`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireViewerField() grx.FieldAuthorizer {
	return func(ctx context.Context, fc grx.FieldAuthorizationContext) error {
		if fc.ParentType != "Query" || fc.FieldName != "viewer" {
			return nil
		}
		if _, ok := session.Subject(ctx); !ok {
			return fmt.Errorf("viewer requires Authorization: Bearer %s", demoBearerToken)
		}
		return nil
	}
}

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
		grx.WithMiddleware(bearerAuth),
		grx.WithFieldAuthorizer(requireViewerField()),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(`GraphQL playground: http://localhost:4010/`)
	log.Println(`For { viewer { id } } add header Authorization: Bearer ` + demoBearerToken)
	log.Fatal(http.ListenAndServe(":4010", srv))
}
