package prompt

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/pr"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	ltable "github.com/charmbracelet/lipgloss/table"
)

const (
	hyperLinkFormat = "\x1b]8;;%s\x07%s\x1b]8;;\x07\u001b[0m"

	white     = lipgloss.Color("#FFFFFF")
	gray      = lipgloss.Color("#CCC9C9")
	lightGray = lipgloss.Color("#959393")
)

var (
	listStyle = lipgloss.NewStyle().Padding(1, 2)

	statusModelStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render

	re          = lipgloss.NewRenderer(os.Stdout)
	CellStyle   = re.NewStyle().Padding(0, 1)
	BorderStyle = lipgloss.NewStyle().Foreground(white).BorderBottom(true)
	HeaderStyle = re.NewStyle().Foreground(white).Bold(true).Align(lipgloss.Center)
)

type prModel struct {
	list           list.Model
	delegateKeys   *delegateKeyMap
	showEventPanel bool
	sections       []string
}

func newModel(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) prModel {
	pullRequests := pr.ListPullRequests(ctx, orgName, repoNames, baseRef, headRef, all)
	items := make([]list.Item, len(pullRequests))
	for i, pr := range pullRequests {
		items[i] = pr
	}

	var delegateKeys = newDelegateKeyMap()

	delegate := newItemDelegate(ctx, orgName, delegateKeys)
	prList := list.New(items, delegate, 0, 0)
	prList.Title = "Interactive Pull Request options"
	prList.Styles.Title = titleStyle

	return prModel{
		list:           prList,
		delegateKeys:   delegateKeys,
		showEventPanel: false,
	}
}

func (m prModel) Init() tea.Cmd {
	return nil
}

func (m prModel) View() string {
	if !m.showEventPanel {
		return listStyle.Render(m.list.View())
	} else {
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			statusModelStyle.Render(lipgloss.JoinVertical(lipgloss.Center, m.sections...)),
		)
	}
}

func (m prModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case eventMsg:
		eventType := msg.eventType
		switch eventType {
		case "STATUS":
			m.showEventPanel = true
			m.sections = statusSections(msg.prResponse, msg.commitResponse, msg.checkRunResponse)
		case "APPROVE":
			m.showEventPanel = true
			// m.message = msg.eventType + " " + msg.message + " APPROVED"
		case "MERGE":
			m.showEventPanel = true
			// m.message = msg.eventType + " " + msg.message + " MERGED"
		case "APPROVE_MERGE":
			m.showEventPanel = true
			// m.message = msg.eventType + " " + msg.message + " APPROVED and MERGED"
		}
	case tea.WindowSizeMsg:
		h, v := listStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		m.showEventPanel = false
		m.sections = nil
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func RunInteractivePR(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) error {
	m := newModel(ctx, orgName, repoNames, baseRef, headRef, all)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func statusSections(prResponse model.PullRequestResponse, commitResponse model.CommitResponse, checkRunResponse model.CheckRunResponse) []string {
	var sections []string

	titleView := lipgloss.NewStyle().Foreground(white).Background(lipgloss.Color("#25A065")).Padding(0, 1).Align(lipgloss.Center)
	responseRows := make([][]string, 0)

	modifiedFiles := make([]string, 0)
	if len(commitResponse.Files) > 0 {
		for _, file := range commitResponse.Files {
			modifiedFiles = append(modifiedFiles, file.Filename)
			if len(modifiedFiles) == 5 {
				modifiedFiles = append(modifiedFiles, "...")
				break
			}
		}
	}

	responseRows = append(responseRows, []string{"PR Number", strconv.Itoa(prResponse.PRNumber)})
	responseRows = append(responseRows, []string{"Repository", prResponse.RepositoryName()})
	responseRows = append(responseRows, []string{"Assignees", strings.ReplaceAll(prResponse.AssigneesName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"Reviewers", strings.ReplaceAll(prResponse.ReviewersName(), "\n", ", ")})
	responseRows = append(responseRows, []string{"Mergeable ?", strconv.FormatBool(prResponse.Mergeable)})
	responseRows = append(responseRows, []string{"Mergeable State", prResponse.MergeableState})
	responseRows = append(responseRows, []string{"Review comments", strconv.Itoa(prResponse.ReviewComments)})
	responseRows = append(responseRows, []string{"Comments", strconv.Itoa(prResponse.Comments)})
	responseRows = append(responseRows, []string{"Commits", strconv.Itoa(prResponse.Commits)})
	responseRows = append(responseRows, []string{"Total files #", strconv.Itoa(prResponse.ChangedFiles)})
	responseRows = append(responseRows, []string{"Modified files in last commit", strings.Join(modifiedFiles, "\n")})
	responseRows = append(responseRows, []string{"Changes", strconv.Itoa(prResponse.Additions) + " Additions, " + strconv.Itoa(prResponse.Deletions) + " Deletions"})
	responseRows = append(responseRows, []string{"Review link", fmt.Sprintf(hyperLinkFormat, prResponse.HTMLUrl, "Open")})

	responseTable := ltable.New().
		Border(lipgloss.HiddenBorder()).
		BorderStyle(BorderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style

			if col == 0 {
				style = CellStyle.Foreground(gray).AlignHorizontal(lipgloss.Right)
			} else {
				style = CellStyle.Foreground(white)
				if responseRows[row-1][0] == "Mergeable ?" && responseRows[row-1][1] == "true" {
					style = style.Foreground(lipgloss.Color("#25A065")).Blink(true)
				} else if responseRows[row-1][0] == "Mergeable ?" && responseRows[row-1][1] == "false" {
					style = style.Foreground(lipgloss.Color("#FF0000"))
				}
			}

			return style
		}).
		Rows(responseRows...)

	responseTableView := lipgloss.NewStyle().BorderForeground(white)
	tableRender := responseTable.String()

	sections = append(sections, titleView.Render(prResponse.Title()))
	sections = append(sections, "\n")
	sections = append(sections, responseTableView.Render(tableRender))

	checkRunRows := make([][]string, 0)
	if len(checkRunResponse.CheckRuns) > 0 {
		for _, checkRun := range checkRunResponse.CheckRuns {
			checkRunRows = append(checkRunRows, []string{checkRun.Name, checkRun.Status, checkRun.Conclusion, checkRun.StartedAt, checkRun.CompletedAt})
		}

		checkRunTable := ltable.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(BorderStyle).
			BorderRow(true).
			StyleFunc(func(row, col int) lipgloss.Style {
				var style lipgloss.Style

				if row == 0 {
					return HeaderStyle
				}
				if col == 0 {
					style = CellStyle.Foreground(gray).AlignHorizontal(lipgloss.Left)
				} else if col == 2 {
					style = CellStyle.Foreground(white)
					if checkRunRows[row-1][2] == "success" {
						style = style.Foreground(lipgloss.Color("#25A065")).Blink(true)
					} else if checkRunRows[row-1][2] == "failure" {
						style = style.Foreground(lipgloss.Color("#FF0000"))
					} else if checkRunRows[row-1][2] == "skipped" {
						style = style.Foreground(lipgloss.Color("#FFA500"))
					}
				}

				return style
			}).
			Headers("Name", "Status", "Conclusion", "Started At", "Completed At").
			Rows(checkRunRows...)

		checkRunTableView := lipgloss.NewStyle().BorderForeground(white)
		sections = append(sections, lipgloss.NewStyle().Foreground(white).Bold(true).Render("Check Runs"))
		sections = append(sections, checkRunTableView.Render(checkRunTable.String()))
	}

	return sections
}
