package backend

import (
	"bytes"
	"testing"
	"time"
)

func TestParseGitLogRecord(t *testing.T) {
	t.Parallel()

	rec := bytes.Join([][]byte{
		[]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		[]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb cccccccccccccccccccccccccccccccccccccccc"),
		[]byte("Alice"),
		[]byte("alice@example.com"),
		[]byte("2024-01-02T03:04:05Z"),
		[]byte("Bob"),
		[]byte("bob@example.com"),
		[]byte("2024-01-02T03:05:06Z"),
		[]byte("Subject line\n\nBody line\n"),
	}, []byte("\n"))

	commit, err := parseGitLogRecord(rec)
	if err != nil {
		t.Fatalf("parseGitLogRecord: %v", err)
	}
	if commit.Hash != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("unexpected hash: %q", commit.Hash)
	}
	if len(commit.ParentHashes) != 2 || commit.ParentHashes[0] != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || commit.ParentHashes[1] != "cccccccccccccccccccccccccccccccccccccccc" {
		t.Fatalf("unexpected parents: %#v", commit.ParentHashes)
	}
	if commit.Author.Name != "Alice" || commit.Author.Email != "alice@example.com" {
		t.Fatalf("unexpected author: %#v", commit.Author)
	}
	if commit.Committer.Name != "Bob" || commit.Committer.Email != "bob@example.com" {
		t.Fatalf("unexpected committer: %#v", commit.Committer)
	}
	if commit.Author.When != (time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("unexpected author time: %v", commit.Author.When)
	}
	if commit.Committer.When != (time.Date(2024, 1, 2, 3, 5, 6, 0, time.UTC)) {
		t.Fatalf("unexpected committer time: %v", commit.Committer.When)
	}
	if commit.Message != "Subject line\n\nBody line\n" {
		t.Fatalf("unexpected message: %q", commit.Message)
	}
}

func TestParseGitLogRecord_EmptyMessage(t *testing.T) {
	t.Parallel()

	rec := []byte("h\n\nan\nae\n2024-01-02T03:04:05Z\ncn\nce\n2024-01-02T03:04:05Z\n")
	commit, err := parseGitLogRecord(rec)
	if err != nil {
		t.Fatalf("parseGitLogRecord: %v", err)
	}
	if commit.Message != "" {
		t.Fatalf("expected empty message, got %q", commit.Message)
	}
}

func TestParseGitLogRecord_ShortRecord(t *testing.T) {
	t.Parallel()

	_, err := parseGitLogRecord([]byte("only\ntwo\nlines"))
	if err == nil {
		t.Fatal("expected error")
	}
}
