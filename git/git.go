package git

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const remoteName = "origin"

type Repo struct {
	path string
	repo *git.Repository
}

func New(path string) (*Repo, error) {
	repo, err := git.PlainOpen(path)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(path, false)
	}
	if err != nil {
		return nil, fmt.Errorf("could not open repository: %w", err)
	}

	return &Repo{path: path, repo: repo}, nil
}

func (r *Repo) UpdateFiles(files map[string][]byte, author *object.Signature) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("could not get worktree: %w", err)
	}

	for fn, buf := range files {
		if err := os.WriteFile(filepath.Join(r.path, fn), buf, 0644); err != nil {
			log.Printf("WARNING: Could not write %s: %v\n", fn, err)
		}
	}

	if err = w.AddGlob("*.conf"); err != nil {
		if errors.Is(err, git.ErrGlobNoMatches) {
			return nil
		}
		return fmt.Errorf("could not stage files: %w", err)
	}

	stat, err := w.Status()
	if err != nil {
		return fmt.Errorf("could not get worktree status: %w", err)
	}
	if stat.IsClean() {
		return nil
	}

	msg := "Update configs:"
	for fn := range stat {
		if strings.HasPrefix(fn, ".git/") {
			continue
		}
		msg += "\n * " + fn
	}

	if _, err = w.Commit(msg, &git.CommitOptions{Author: author}); err != nil {
		return fmt.Errorf("could not get commit worktree: %w", err)
	}

	return nil
}

func (r *Repo) PushRemote(url string, auth transport.AuthMethod) error {
	remotes, err := r.repo.Remotes()
	if err != nil {
		return fmt.Errorf("could not get remotes: %w", err)
	}
	for _, rem := range remotes {
		if rem.Config().Name == remoteName {
			if err := r.repo.DeleteRemote(remoteName); err != nil {
				return fmt.Errorf("could not delete remote: %w", err)
			}
		}
	}
	rem, err := r.repo.CreateRemote(&config.RemoteConfig{
		Name:  remoteName,
		URLs:  []string{url},
		Fetch: []config.RefSpec{"+refs/heads/*:refs/remotes/origin/*"},
	})
	if err != nil {
		return fmt.Errorf("could not create remote: %w", err)
	}

	err = rem.Push(&git.PushOptions{RemoteName: remoteName, Auth: auth})
	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return fmt.Errorf("could not push remote: %w", err)
}
