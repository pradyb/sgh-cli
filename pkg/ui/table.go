package ui

import (
	"fmt"
	"os"
	"strconv"

	"github.com/prady-lab/sgh-cli/internal/model"

	"github.com/aquasecurity/table"
	"github.com/liamg/tml"
)

const cyanFormat = "<cyan>%s</cyan>"

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
			openIssues = tml.Sprintf("<red>%s</red>", openIssues)
		}
		t.AddRow(strconv.Itoa(repo.Id), tml.Sprintf("<green>%s</green>", repo.Name), repo.Description, repo.Language, repo.SSHUrl, openIssues, strconv.Itoa(repo.Size))
	}
	t.Render()
}

func PrintResponses(responses []model.CommonResponse) {
	fmt.Println()
	t := table.New(os.Stdout)
	t.SetHeaders("Repository", "Status Message")
	t.SetHeaderStyle(table.StyleBold)
	t.SetLineStyle(table.StyleBrightWhite)
	t.SetDividers(table.UnicodeRoundedDividers)
	t.SetFooters(tml.Sprintf(cyanFormat, "Total Repositories"), tml.Sprintf(cyanFormat, strconv.Itoa(len(responses))))

	for _, response := range responses {
		message := tml.Sprintf("<green>%s</green>", response.SuccessMessage)
		if response.ErrorMessage != "" {
			message = tml.Sprintf("<red>%s</red>", response.ErrorMessage)
		}
		t.AddRow(response.RepositoryName, message)
	}
	t.Render()
}
