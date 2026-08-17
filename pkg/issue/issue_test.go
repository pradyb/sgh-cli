// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"github.com/prady-lab/sgh-cli/pkg/context"
)

func TestBuildIssueSearchQuery(t *testing.T) {
	tests := []struct {
		name string
		req  IssueListRequest
		want string
	}{
		{
			name: "org scope with defaults",
			req:  IssueListRequest{OrgName: "sample-org", State: "open"},
			want: "org:sample-org is:issue state:open",
		},
		{
			name: "author maps to the author search qualifier",
			req:  IssueListRequest{OrgName: "sample-org", State: "open", Author: "jane-doe"},
			want: "org:sample-org is:issue state:open author:jane-doe",
		},
		{
			name: "all filters",
			req: IssueListRequest{
				OrgName:  "sample-org",
				State:    "closed",
				Labels:   "bug",
				Assignee: "john-doe",
				Author:   "jane-doe",
			},
			want: "org:sample-org is:issue state:closed label:bug assignee:john-doe author:jane-doe",
		},
		{
			name: "state all is omitted",
			req:  IssueListRequest{OrgName: "sample-org", State: "all", Author: "jane-doe"},
			want: "org:sample-org is:issue author:jane-doe",
		},
	}

	ctx := &context.Context{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildIssueSearchQuery(ctx, tt.req); got != tt.want {
				t.Errorf("buildIssueSearchQuery() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}
