package apperrors

import "fmt"

type GitHubError struct {
	StatusCode int
	Message    string
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("%d - %s", e.StatusCode, e.Message)
}
