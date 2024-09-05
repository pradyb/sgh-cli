package model

var SearchRepository struct {
	Search struct {
		RepositoryCount int
		PageInfo        struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Repository struct {
					Name             string
					NameWithOwner    string
					Url              string
					SSHUrl           string
					Description      string
					IsPrivate        bool
					IsArchived       bool
					IsDisabled       bool
					DefaultBranchRef struct {
						Name string
					}
					PrimaryLanguage struct {
						Name string
					}
					PullRequests struct {
						TotalCount int
					} `graphql:"pullRequests(first:1, states: OPEN)"`
					Issues struct {
						TotalCount int
					} `graphql:"issues(first:1, states: OPEN)"`
				} `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $repoCursor)"`
}

var SearchPullRequests struct {
	Search struct {
		IssueCount int
		PageInfo   struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				PullRequest struct {
					Number  int
					Title   string
					Url     string
					Body    string
					BaseRef struct {
						Name       string
						Repository struct {
							Name string
						}
					}
					BaseRefName string
					HeadRef     struct {
						Name       string
						Repository struct {
							Name string
						}
					}
					HeadRefName string
					Author      struct {
						Login string
					}
					ReviewRequests struct {
						TotalCount int
						Edges      []struct {
							Node struct {
								RequestedReviewer struct {
									User struct {
										Login string
										Name  string
									} `graphql:"... on User"`
								}
							}
						}
					} `graphql:"reviewRequests(first: 1)"`
				} `graphql:"... on PullRequest"`
			}
		}
	} `graphql:"search(query: $queryString, type: ISSUE, last: 20, after: $prCursor)"`
}

var PullRequestDetailsQuery struct {
	Organization struct {
		Repository struct {
			PullRequest struct {
				Number  int
				Title   string
				Body    string
				Url     string
				BaseRef struct {
					Name       string
					Repository struct {
						Name string
					}
				}
				BaseRefName string
				HeadRef     struct {
					Name       string
					Repository struct {
						Name string
					}
				}
				HeadRefName      string
				ReviewDecision   string
				State            string
				Mergeable        string
				MergeStateStatus string
				MergedAt         string
				MergedBy         struct {
					Login string
				}
				Author struct {
					Login string
				}
				Repository struct {
					Name string
				}
				ReviewRequests struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							RequestedReviewer struct {
								User struct {
									Login string
									Name  string
								} `graphql:"... on User"`
							}
						}
					}
				} `graphql:"reviewRequests(first: 50)"`
				Assignees struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Login string
							Name  string
						}
					}
				} `graphql:"assignees(first: 10)"`
				TotalCommentsCount int
				Comments           struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Author struct {
								Login string
							}
							Body      string
							CreatedAt string
						}
					}
				} `graphql:"comments(first: 10)"`
				Commits struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Commit struct {
								CheckSuites struct {
									TotalCount int
									Edges      []struct {
										Node struct {
											Conclusion string
											CheckRuns  struct {
												TotalCount int
												Edges      []struct {
													Node struct {
														Status      string
														Conclusion  string
														StartedAt   string
														CompletedAt string
														DetailsUrl  string
														Name        string
														Text        string
														Summary     string
														Title       string
													}
												}
											} `graphql:"checkRuns(first: 10)"`
										}
									}
								} `graphql:"checkSuites(first: 10)"`
							}
						}
					}
				} `graphql:"commits(last: 1)"`
				Additions    int
				Deletions    int
				ChangedFiles int
				Files        struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Path       string
							Additions  int
							Deletions  int
							ChangeType string
						}
					}
				} `graphql:"files(first: 10)"`
				Reviews struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Author struct {
								Login string
							}
							State       string
							Body        string
							CreatedAt   string
							SubmittedAt string
							Commit      struct {
								Oid string
								Id  string
							}
						}
					}
				} `graphql:"reviews(first: 10)"`
			} `graphql:"pullRequest(number: $prNumber)"`
		} `graphql:"repository(name: $repoName)"`
	} `graphql:"organization(login: $orgName)"`
}
