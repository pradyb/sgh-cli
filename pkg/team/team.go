package team

import (
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/shurcooL/githubv4"
)

var q struct {
	Organization struct {
		Name  string
		Url   string
		Teams struct {
			Edges []struct {
				Node struct {
					Name    string
					Url     string
					Members struct {
						TotalCount int
						Edges      []struct {
							Node struct {
								Login string
								Name  string
								Url   string
							}
						}
						PageInfo struct {
							EndCursor   githubv4.String
							HasNextPage bool
						}
					} `graphql:"members(first: $noOfMembers, after: $memberCursor, orderBy: {field: LOGIN, direction: ASC})"`
				}
			}
		} `graphql:"teams(first: 10, query: $teamName)"`
	} `graphql:"organization(login: $orgName)"`
}

func GetTeamAndMembers(ctx *context.Context, orgName string, teamName string, noOfMembers int, allMembers bool) ([]model.Team, error) {
	variables := map[string]interface{}{
		"orgName":      githubv4.String(orgName),
		"noOfMembers":  githubv4.Int(noOfMembers),
		"teamName":     githubv4.String(teamName),
		"memberCursor": (*githubv4.String)(nil),
	}

	teams := make([]model.Team, 0)

	var currentTeamName string
	var currentMembers = make([]model.Member, 0)
	var currentTeamNextPage githubv4.String

	for {
		err := service.Query(ctx, &q, variables)
		if err != nil {
			return nil, err
		}

		for _, team := range q.Organization.Teams.Edges {
			if currentTeamName == "" {
				currentTeamName = team.Node.Name
			}
			if currentTeamName == team.Node.Name {
				for _, member := range team.Node.Members.Edges {
					currentMembers = append(currentMembers, model.Member{
						Login:     member.Node.Login,
						Name:      member.Node.Name,
						Url:       member.Node.Url,
						PeopleUrl: strings.ReplaceAll(string(q.Organization.Url), q.Organization.Name, "orgs/"+q.Organization.Name) + "/people/" + member.Node.Login,
					})
				}
				if allMembers && team.Node.Members.PageInfo.HasNextPage {
					currentTeamNextPage = team.Node.Members.PageInfo.EndCursor
					break
				} else {
					teams = append(teams, model.Team{
						Name:         currentTeamName,
						TotalMembers: team.Node.Members.TotalCount,
						Url:          team.Node.Url,
						Members:      currentMembers,
					})
					currentTeamNextPage = *githubv4.NewString("")
					currentTeamName = ""
					currentMembers = make([]model.Member, 0)
				}
			}
		}
		if currentTeamNextPage == "" {
			break
		}
		variables["memberCursor"] = currentTeamNextPage
	}
	return teams, nil
}
