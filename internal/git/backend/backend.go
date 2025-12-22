package backend

// Backend abstracts access to repository data.
//
// The default implementation shells out to the git executable, but the interface
// allows alternative implementations (e.g. pure-Go) without changing callers.
type Backend interface {
	RepoPath() string
	StartLogStream(fromHash string) (LogStream, error)

	HeadState() (hash string, headName string, ok bool, err error)
	ListRefs() ([]Ref, error)

	CommitDiffText(commitHash string, parentHash string) (string, error)
	WorktreeDiffText(staged bool) (string, error)
	LocalChangesStatus() (LocalChanges, error)
}

type LogStream interface {
	Next() (*Commit, error)
	Close() error
}
