package apperrors

type GitHubError struct {
	StatusCode int
	Message    string
}

func (e *GitHubError) Error() string {
	return e.Message
}
