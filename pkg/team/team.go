// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

// Package team provides functions for interacting with GitHub teams and their members.
// It supports listing teams, retrieving team members, and related operations for organizations.
package team

import (
	"fmt"
	"strings"

	"github.com/shurcooL/githubv4"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

// TeamMembersRequest contains parameters for getting team and members information.
type TeamMembersRequest struct {
	OrgName     string
	TeamName    string
	NoOfMembers int
	AllMembers  bool
}

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
			PageInfo model.PageInfo
		} `graphql:"teams(first: $teamPageSize, after: $teamCursor, query: $teamName)"`
	} `graphql:"organization(login: $orgName)"`
}

func GetTeamAndMembers(ctx *context.Context, req TeamMembersRequest) ([]model.OrgTeam, error) {
	variables := map[string]interface{}{
		"orgName":      githubv4.String(req.OrgName),
		"noOfMembers":  githubv4.Int(req.NoOfMembers),
		"teamName":     githubv4.String(req.TeamName),
		"teamPageSize": githubv4.Int(100),
		"teamCursor":   (*githubv4.String)(nil),
		"memberCursor": (*githubv4.String)(nil),
	}

	teams := make([]model.OrgTeam, 0)

	currentMembers := make([]model.OrgTeamMember, 0)
	var currentTeamNextPage string

	for {
		var q TeamQuery
		err := service.Query(ctx, &q, variables)
		if err != nil {
			return nil, fmt.Errorf("failed to query team members: %w", err)
		}

		for _, team := range q.Organization.Teams.Edges {
			currentMembers = append(currentMembers, getMembers(team.Node.Members, strings.ReplaceAll(q.Organization.Url, q.Organization.Name, "orgs/"+q.Organization.Name)+"/people/")...)
			if req.AllMembers && team.Node.Members.PageInfo.HasNextPage {
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

		if currentTeamNextPage != "" {
			// Paginating members within a team — keep team cursor unchanged
			variables["memberCursor"] = githubv4.String(currentTeamNextPage)
			continue
		}

		// Advance to next page of teams if available
		if q.Organization.Teams.PageInfo.HasNextPage {
			variables["teamCursor"] = githubv4.String(q.Organization.Teams.PageInfo.EndCursor)
			variables["memberCursor"] = (*githubv4.String)(nil)
		} else {
			break
		}
	}
	return teams, nil
}

func getMembers(members Members, peopleBaseUrl string) []model.OrgTeamMember {
	teamMembers := make([]model.OrgTeamMember, 0)
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
