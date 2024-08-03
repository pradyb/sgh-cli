package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"

	"github.com/aquasecurity/table"
	"github.com/liamg/tml"
)

const cyanFormat = "<cyan>%s</cyan>"
const redFormat = "<red>%s</red>"
const greenFormat = "<green>%s</green>"
const blueFormat = "<blue>%s</blue>"
const yellowFormat = "<yellow>%s</yellow>"

func PrintRepositories(repos []model.Repository) {
	fmt.Println()
	t := table.New(os.Stdout)
	t.SetHeaders("Id", "Name", "Description", "Language", "SSH URL", "Open Issues", "Size")
	t.SetHeaderStyle(table.StyleBold)
	t.SetLineStyle(table.StyleBrightWhite)
	t.SetDividers(table.UnicodeRoundedDividers)
	t.SetFooters(tml.Sprintf(cyanFormat, "Total Repositories"), tml.Sprintf(cyanFormat, strconv.Itoa(len(repos))))

	for _, repo := range repos {
		openIssues := strconv.Itoa(repo.OpenIssuesCount)
		if repo.OpenIssuesCount > 0 {
			openIssues = tml.Sprintf(redFormat, openIssues)
		}
		t.AddRow(strconv.Itoa(repo.Id), tml.Sprintf(greenFormat, repo.Name), repo.Description, repo.Language, repo.SSHUrl, openIssues, strconv.Itoa(repo.Size))
	}
	t.Render()
}

func PrintResponses(responses []model.RefUIResponse) {
	fmt.Println()
	t := table.New(os.Stdout)
	t.SetHeaders("Repository", "Status Message")
	t.SetHeaderStyle(table.StyleBold)
	t.SetLineStyle(table.StyleBrightWhite)
	t.SetDividers(table.UnicodeRoundedDividers)
	t.SetFooters(tml.Sprintf(cyanFormat, "Total Repositories"), tml.Sprintf(cyanFormat, strconv.Itoa(len(responses))))

	for _, response := range responses {
		message := tml.Sprintf(greenFormat, response.SuccessMessage)
		if response.ErrorMessage != "" {
			message = tml.Sprintf(redFormat, response.ErrorMessage)
		}
		t.AddRow(response.RepositoryName, message)
	}
	t.Render()
}

func PrintPullRequestResponses(prResponses []model.PullRequestResponse) {
	fmt.Println()
	t := table.New(os.Stdout)
	t.SetHeaders("Id", "Repository", "Title", "Created User", "Assignees", "Reviewers", "Status", "Refs", "HTMLUrl")
	t.SetHeaderStyle(table.StyleBold)
	t.SetLineStyle(table.StyleBrightWhite)
	t.SetDividers(table.UnicodeRoundedDividers)
	t.SetFooters(tml.Sprintf(cyanFormat, "Total Pull Requests"), tml.Sprintf(cyanFormat, strconv.Itoa(len(prResponses))))
	//t.SetFooterColSpans(0, 5, 3)

	for _, pr := range prResponses {
		status := tml.Sprintf(yellowFormat, pr.Status)
		if strings.EqualFold("closed", pr.Status) {
			status = tml.Sprintf(greenFormat, status)
		}
		refs := fmt.Sprintf("%s <- %s", pr.Base.Ref, pr.Head.Ref)
		if pr.ErrorMessage != "" {
			t.AddRow(pr.RepositoryName(), pr.ErrorMessage)
		} else {
			t.AddRow(strconv.Itoa(pr.PRNumber), tml.Sprintf(greenFormat, pr.RepositoryName()), pr.Title, pr.UserName(), pr.AssigneesName(), pr.ReviewersName(), status, refs, pr.HTMLUrl)
		}
	}
	t.Render()
}
