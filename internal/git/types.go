package git

import "time"

type Signature struct {
	Name  string
	Email string
	When  time.Time
}

type Commit struct {
	Hash         string
	ParentHashes []string
	Author       Signature
	Committer    Signature
	Message      string
}
