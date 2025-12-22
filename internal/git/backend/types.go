package backend

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

type LocalChanges struct {
	HasWorktree bool
	HasStaged   bool
}

type RefKind uint8

const (
	RefKindBranch RefKind = iota
	RefKindRemoteBranch
	RefKindTag
)

type Ref struct {
	Hash string
	Kind RefKind
	Name string // short name: main, origin/main, v1
}
