package pralerts

import (
	"strings"

	gh "github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
)

type snapshotQuery struct {
	Search struct {
		PageInfo struct {
			HasNextPage bool
			EndCursor   string
		}
		Nodes []struct {
			PullRequest struct {
				ID         string
				Number     int
				Title      string
				Repository struct {
					NameWithOwner string
				}
				Commits struct {
					Nodes []struct {
						Commit struct {
							StatusCheckRollup struct {
								State string
							}
						}
					}
				} `graphql:"commits(last: 1)"`
				Comments struct {
					Nodes []struct {
						ID     string
						Author struct {
							Login string
						}
					}
				} `graphql:"comments(last: 50)"`
				ReviewThreads struct {
					Nodes []struct {
						Comments struct {
							Nodes []struct {
								ID     string
								Author struct {
									Login string
								}
							}
						} `graphql:"comments(last: 50)"`
					}
				} `graphql:"reviewThreads(last: 50)"`
				Reviews struct {
					Nodes []struct {
						ID     string
						State  string
						Author struct {
							Login string
						}
					}
				} `graphql:"reviews(last: 50)"`
			} `graphql:"... on PullRequest"`
		}
	} `graphql:"search(type: ISSUE, first: 50, after: $endCursor, query: $query)"`
}

// Snapshot returns the open PRs authored by, requested from or reviewed by the
// authenticated user, with their CI state and foreign comment/review events.
func Snapshot() ([]PR, error) {
	client, err := gh.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}
	var viewer struct {
		Viewer struct {
			Login string
		}
	}
	if err := client.Query("PRAlertsViewer", &viewer, nil); err != nil {
		return nil, err
	}
	user := viewer.Viewer.Login

	prs := map[string]PR{}
	for _, search := range []string{
		"is:pr is:open author:@" + user,
		"is:pr is:open review-requested:@" + user,
		"is:pr is:open reviewed-by:@" + user,
	} {
		cursor := (*graphql.String)(nil)
		for {
			vars := map[string]any{"query": graphql.String(search), "endCursor": cursor}
			var result snapshotQuery
			if err := client.Query("PRAlertsSnapshot", &result, vars); err != nil {
				return nil, err
			}
			for _, node := range result.Search.Nodes {
				pr := node.PullRequest
				if pr.ID == "" {
					continue
				}
				var events []Event
				seen := map[string]bool{}
				addEvent := func(id, kind, actor string) {
					if id == "" || actor == "" || strings.EqualFold(actor, user) || seen[id] {
						return
					}
					seen[id] = true
					events = append(events, Event{ID: id, Kind: kind, Actor: actor})
				}
				for _, comment := range pr.Comments.Nodes {
					addEvent(comment.ID, "Comentario", comment.Author.Login)
				}
				for _, thread := range pr.ReviewThreads.Nodes {
					for _, comment := range thread.Comments.Nodes {
						addEvent(comment.ID, "Comentario", comment.Author.Login)
					}
				}
				for _, review := range pr.Reviews.Nodes {
					switch review.State {
					case "APPROVED":
						addEvent(review.ID, "Aprobación", review.Author.Login)
					case "CHANGES_REQUESTED":
						addEvent(review.ID, "Cambios solicitados", review.Author.Login)
					}
				}
				ci := ""
				if len(pr.Commits.Nodes) > 0 {
					ci = pr.Commits.Nodes[0].Commit.StatusCheckRollup.State
				}
				existing, ok := prs[pr.ID]
				if !ok {
					existing = PR{ID: pr.ID, Repo: pr.Repository.NameWithOwner, Number: pr.Number, Title: pr.Title, CI: ci}
				}
				existing.Events = append(existing.Events, events...)
				prs[pr.ID] = existing
			}
			if !result.Search.PageInfo.HasNextPage {
				break
			}
			c := graphql.String(result.Search.PageInfo.EndCursor)
			cursor = &c
		}
	}
	out := make([]PR, 0, len(prs))
	for _, pr := range prs {
		out = append(out, pr)
	}
	return out, nil
}
