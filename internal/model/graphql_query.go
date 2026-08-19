// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package model

type (
	PageInfo struct {
		StartCursor     string
		HasPreviousPage bool
		EndCursor       string
		HasNextPage     bool
	}

	OrganizationFragment struct {
		Description string
	}

	SimpleRepositoryFragment struct {
		Name          string
		NameWithOwner string
		Url           string
		SSHUrl        string
	}

	ActorFragment struct {
		User UserFragment `graphql:"... on User"`
	}

	UserFragment struct {
		Login      string
		Name       string
		Bio        string
		WebsiteUrl string
	}

	TeamFragment struct {
		Name        string
		Description string
	}

	RefFragment struct {
		Name       string
		Repository SimpleRepositoryFragment
	}

	ReviewRequestFragment struct {
		RequestedReviewer struct {
			Type string       `graphql:"__typename"`
			User UserFragment `graphql:"... on User"`
			Team TeamFragment `graphql:"... on Team"`
		}
	}
	ReviewRequestsFragment struct {
		TotalCount int
		Edges      []struct {
			Node ReviewRequestFragment
		}
	}

	AssigneesFragment struct {
		TotalCount int
		Edges      []struct {
			Node UserFragment
		}
	}

	CommentFragment struct {
		Author    ActorFragment
		Body      string
		CreatedAt string
	}

	CommentsFragment struct {
		TotalCount int
		Edges      []struct {
			Node CommentFragment
		}
	}
)

type SearchRepositoriesQuery struct {
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
					/*Issues struct {
						TotalCount int
					} `graphql:"issues(first:1, states: OPEN)"`*/
				} `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $repoCursor)"`
}

type SearchPullRequestsQuery struct {
	Search struct {
		IssueCount int
		PageInfo   struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				PullRequest struct {
					Number           int
					Title            string
					Url              string
					Body             string
					BaseRef          RefFragment `graphql:"baseRef"`
					BaseRefName      string
					HeadRef          RefFragment `graphql:"headRef"`
					HeadRefName      string
					State            string
					MergeStateStatus string
					Author           ActorFragment
					ReviewRequests   ReviewRequestsFragment `graphql:"reviewRequests(first: 3)"`
					Assignees        AssigneesFragment      `graphql:"assignees(first: 3)"`
				} `graphql:"... on PullRequest"`
			}
		}
	} `graphql:"search(query: $queryString, type: ISSUE, last: $lastCount, after: $prCursor)"`
}

type PullRequestDetailQuery struct {
	Organization struct {
		Repository struct {
			PullRequest struct {
				Number           int
				Title            string
				Body             string
				Url              string
				BaseRef          RefFragment `graphql:"baseRef"`
				BaseRefName      string
				HeadRef          RefFragment `graphql:"headRef"`
				HeadRefName      string
				HeadRefOid       string
				ReviewDecision   string
				State            string
				Mergeable        string
				MergeStateStatus string
				CreatedAt        string
				UpdatedAt        string
				MergedAt         string
				MergedBy         ActorFragment
				Author           ActorFragment
				Repository       SimpleRepositoryFragment
				ReviewRequests   ReviewRequestsFragment `graphql:"reviewRequests(first: 50)"`
				Assignees        AssigneesFragment      `graphql:"assignees(first: 10)"`
				Labels           struct {
					TotalCount int
					Edges      []struct {
						Node struct {
							Name  string
							Color string
						}
					}
				} `graphql:"labels(first: 10)"`
				TotalCommentsCount int
				Comments           CommentsFragment `graphql:"comments(first: 10)"`
				Commits            struct {
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
							Author      ActorFragment
							State       string
							Body        string
							CreatedAt   string
							SubmittedAt string
							Commit      struct {
								Oid string
								ID  string
							}
						}
					}
				} `graphql:"reviews(first: 10)"`
			} `graphql:"pullRequest(number: $prNumber)"`
		} `graphql:"repository(name: $repoName)"`
	} `graphql:"organization(login: $orgName)"`
}

type SearchProtectedBranchesQuery struct {
	Search struct {
		RepositoryCount int
		PageInfo        struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Repository ProtectedBranchRepoFragment `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $repoCursor)"`
}

type ProtectedBranchRepoFragment struct {
	Name string
	Refs struct {
		TotalCount int
		Edges      []struct {
			Node ProtectedBranchRefFragment
		}
	} `graphql:"refs(query: $branchName, refPrefix: \"refs/heads/\", first: 20)"`
}

type ProtectedBranchRefFragment struct {
	Name                 string
	Rules                RulesEdgeFragment `graphql:"rules(first: 5)"`
	BranchProtectionRule ProtectedBranchRuleFragment
}

type ProtectedBranchRuleFragment struct {
	AllowsDeletions                bool
	AllowsForcePushes              bool
	BlocksCreations                bool
	DismissesStaleReviews          bool
	IsAdminEnforced                bool
	LockAllowsFetchAndMerge        bool
	LockBranch                     bool
	Pattern                        string
	RequireLastPushApproval        bool
	RequiredApprovingReviewCount   int
	RequiredDeploymentEnvironments []string
	RequiresApprovingReviews       bool
	RequiresCodeOwnerReviews       bool
	RequiresCommitSignatures       bool
	RequiresConversationResolution bool
	RequiresDeployments            bool
	RequiresLinearHistory          bool
	RequiresStatusChecks           bool
	RequiresStrictStatusChecks     bool
	RestrictsPushes                bool
	RestrictsReviewDismissals      bool
	RequiredStatusChecks           []struct {
		Context string
	}
	PushAllowances struct {
		TotalCount int
		Edges      []struct {
			Node struct {
				Actor ActorFragment
			}
		}
	} `graphql:"pushAllowances(first: 10)"`
	BypassPullRequestAllowances struct {
		TotalCount int
		Edges      []struct {
			Node struct {
				Actor ActorFragment
			}
		}
	} `graphql:"bypassPullRequestAllowances(first: 10)"`
}

type RulesEdgeFragment struct {
	TotalCount int
	Edges      []struct {
		Node RuleFragment
	}
}

type RuleFragment struct {
	ID                string
	Type              string
	Parameters        RuleParameters
	RepositoryRuleset struct {
		Name         string
		Enforcement  string
		BypassActors struct {
			TotalCount int
			Edges      []struct {
				Node struct {
					Actor struct {
						Team TeamFragment `graphql:"... on Team"`
					}
				}
			}
		} `graphql:"bypassActors(first: 1)"`
	}
}

type RuleParameters struct {
	TypeName                  string                     `graphql:"__typename"`
	RequiredStatusChecksParam RuleParamStatusChecksParam `graphql:"... on RequiredStatusChecksParameters"`
	PullRequestParam          RulePullRequestParam       `graphql:"... on PullRequestParameters"`
}

type RuleParamStatusChecksParam struct {
	RequiredStatusChecks []struct {
		Context string
	}
}

type RulePullRequestParam struct {
	RequiredApprovingReviewCount   int
	DismissStaleReviewsOnPush      bool
	RequireCodeOwnerReview         bool
	RequireLastPushApproval        bool
	RequiredReviewThreadResolution bool
}

type ReviewPullRequestQuery struct {
	Repository struct {
		PullRequest struct {
			Reviews struct {
				Edges []struct {
					Node struct {
						ID          string
						Author      User
						State       string
						Body        string
						CreatedAt   string
						SubmittedAt string
						Commit      struct {
							Oid string
						}
					}
				}
				PageInfo PageInfo
			} `graphql:"reviews(first: $noOfReviews, after: $reviewCursor, orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"pullRequest(number: $prNumber)"`
	} `graphql:"repository(owner: $owner, name: $repoName)"`
}

type SearchIssuesQuery struct {
	Search struct {
		IssueCount int
		PageInfo   struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Issue struct {
					Number     int
					Title      string
					Url        string
					Body       string
					State      string
					CreatedAt  string
					UpdatedAt  string
					Author     ActorFragment
					Repository SimpleRepositoryFragment
					Assignees  AssigneesFragment `graphql:"assignees(first: 3)"`
					Labels     struct {
						TotalCount int
						Edges      []struct {
							Node struct {
								Name string
							}
						}
					} `graphql:"labels(first: 5)"`
					Comments struct {
						TotalCount int
					}
				} `graphql:"... on Issue"`
			}
		}
	} `graphql:"search(query: $queryString, type: ISSUE, last: $lastCount, after: $issueCursor)"`
}

type SearchBranchesQuery struct {
	Search struct {
		RepositoryCount int
		PageInfo        struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Repository struct {
					Name string
					Refs struct {
						TotalCount int
						Edges      []struct {
							Node struct {
								Name                 string
								Target               struct{ Oid string }
								BranchProtectionRule struct {
									IsAdminEnforced bool
									Pattern         string
								}
							}
						}
					} `graphql:"refs(query: $branchFilter, refPrefix: \"refs/heads/\", first: 50)"`
				} `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $branchCursor)"`
}

type SearchTagsQuery struct {
	Search struct {
		RepositoryCount int
		PageInfo        struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Repository struct {
					Name string
					Refs struct {
						TotalCount int
						Edges      []struct {
							Node struct {
								Name   string
								Target struct {
									Oid string
								}
							}
						}
					} `graphql:"refs(query: $tagFilter, refPrefix: \"refs/tags/\", first: 50)"`
				} `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $tagCursor)"`
}
