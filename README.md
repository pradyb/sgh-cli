# Simple Github CLI


```sh
sgh config add org prady-github-api

sgh config add pattern "(.)*" -o prady-github-api -i

sgh config set tagger-name "Pradeep Kumar Balakrishnan" -o prady-github-api
sgh config set tagger-email "b.pradeepkumar@gmail.com" -o prady-github-api
```


```json
{
    "name": "prady-github-api",
    "repositories": [
        "user-service",
        "admin-service",
        "public-repo"
    ],
    "repo_patterns": {
        "include": [
            "(.)*"
        ]
    },
    "tagger": {
        "name": "Pradeep Kumar Balakrishnan",
        "email": "b.pradeepkumar@gmail.com"
    },
    "protected_branch": {
        "bypass_pull_request_users": [
            "pradyb"
        ],
        "allowed_restrictions_users": [
            "pradyb"
        ],
        "approving_review_count": 1
    }
}
```