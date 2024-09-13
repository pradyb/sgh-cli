package team

import (
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/shurcooL/githubv4"
)

type Members struct {
	TotalCount int
	Edges      []struct {
		Node model.UserFragment
	}
	PageInfo model.PageInfo
}

type TeamQuery struct {
	Organization struct {
		Name  string
		Url   string
		Teams struct {
			Edges []struct {
				Node struct {
					Name         string
					Url          string
					Members      Members `graphql:"members(first: $noOfMembers, after: $memberCursor, orderBy: {field: LOGIN, direction: ASC})"`
					Repositories struct {
						TotalCount int
					} `graphql:"repositories(first: 1)"`
				}
			}
		} `graphql:"teams(first: 10, query: $teamName)"`
	} `graphql:"organization(login: $orgName)"`
}

func GetTeamAndMembers(ctx *context.Context, orgName string, teamName string, noOfMembers int, allMembers bool) ([]model.OrgTeam, error) {
	variables := map[string]interface{}{
		"orgName":      githubv4.String(orgName),
		"noOfMembers":  githubv4.Int(noOfMembers),
		"teamName":     githubv4.String(teamName),
		"memberCursor": (*githubv4.String)(nil),
	}

	teams := make([]model.OrgTeam, 0)

	var currentMembers = make([]model.OrgTeamMember, 0)
	var currentTeamNextPage string

	for {
		var q TeamQuery
		err := service.Query(ctx, &q, variables)
		if err != nil {
			return nil, err
		}

		for _, team := range q.Organization.Teams.Edges {
			currentMembers = append(currentMembers, getMembers(team.Node.Members, strings.ReplaceAll(string(q.Organization.Url), q.Organization.Name, "orgs/"+q.Organization.Name)+"/people/")...)
			if allMembers && team.Node.Members.PageInfo.HasNextPage {
				currentTeamNextPage = team.Node.Members.PageInfo.EndCursor
				break
			} else {
				teams = append(teams, model.OrgTeam{
					Name:              team.Node.Name,
					TotalMembers:      team.Node.Members.TotalCount,
					Url:               team.Node.Url,
					Members:           currentMembers,
					RepositoriesCount: team.Node.Repositories.TotalCount,
				})
				currentTeamNextPage = ""
				currentMembers = make([]model.OrgTeamMember, 0)
			}
		}
		if currentTeamNextPage == "" {
			break
		}
		variables["memberCursor"] = githubv4.String(currentTeamNextPage)
	}
	return teams, nil
}

func getMembers(members Members, peopleBaseUrl string) []model.OrgTeamMember {
	var teamMembers = make([]model.OrgTeamMember, 0)
	for _, member := range members.Edges {
		teamMembers = append(teamMembers, model.OrgTeamMember{
			Login:     member.Node.Login,
			Name:      member.Node.Name,
			Url:       member.Node.WebsiteUrl,
			PeopleUrl: peopleBaseUrl + member.Node.Login,
		})
	}
	return teamMembers
}
