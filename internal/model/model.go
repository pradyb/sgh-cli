package model

type Repositories struct {
	Repositories []Repository
}

type Repository struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	Private         bool   `json:"private"`
	Description     string `json:"description"`
	HTMLUrl         string `json:"html_url"`
	SSHUrl          string `json:"ssh_url"`
	CloneUrl        string `json:"clone_url"`
	OpenIssuesCount int    `json:"open_issues_count"`
	Language        string `json:"language"`
	Size            int    `json:"size"`
}

type BranchRequest struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type NewItemResponse struct {
	Ref    string `json:"ref"`
	Url    string `json:"url"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
		Url  string `json:"url"`
	} `json:"object"`
}

type TagRequest struct {
	Tag     string `json:"tag"`
	Object  string `json:"object"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Tagger  Tagger `json:"tagger"`
}

type Tagger struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CommonResponse struct {
	OrgName        string
	RepositoryName string
	ItemName       string
	ItemType       string
	SuccessMessage string
	ErrorMessage   string
}

type PRResponse struct {
	PRNumber     int    `json:"number"`
	Title        string `json:"title"`
	Status       string `json:"state"`
	HTMLUrl      string `json:"html_url"`
	ErrorMessage string
}
