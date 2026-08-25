package bitbucket

import (
	"fmt"
	"sort"
	"strings"
)

// Fields lists the --json field names available for a type, mapped to the
// function that extracts the value from an instance. Keeping the map here,
// beside the struct, makes it easy to keep in sync (see fields_test.go).
type Fields[T any] map[string]func(T) any

// Names returns the sorted field names.
func (f Fields[T]) Names() []string {
	names := make([]string, 0, len(f))
	for k := range f {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Validate ensures every requested field exists.
func (f Fields[T]) Validate(requested []string) error {
	var unknown []string
	for _, r := range requested {
		if _, ok := f[r]; !ok {
			unknown = append(unknown, r)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown JSON field(s): %s\nAvailable fields:\n  %s",
			strings.Join(unknown, ", "), strings.Join(f.Names(), "\n  "))
	}
	return nil
}

// Export projects v onto the requested fields (all fields when empty).
func (f Fields[T]) Export(v T, requested []string) map[string]any {
	if len(requested) == 0 {
		requested = f.Names()
	}
	out := make(map[string]any, len(requested))
	for _, r := range requested {
		if fn, ok := f[r]; ok {
			out[r] = fn(v)
		}
	}
	return out
}

// ExportAll projects each element of vs.
func (f Fields[T]) ExportAll(vs []T, requested []string) []map[string]any {
	out := make([]map[string]any, 0, len(vs))
	for _, v := range vs {
		out = append(out, f.Export(v, requested))
	}
	return out
}

// RepositoryFields are the exportable fields for repositories.
var RepositoryFields = Fields[Repository]{
	"name":        func(r Repository) any { return r.Name },
	"slug":        func(r Repository) any { return r.Slug },
	"fullName":    func(r Repository) any { return r.FullName },
	"description": func(r Repository) any { return r.Description },
	"isPrivate":   func(r Repository) any { return r.IsPrivate },
	"language":    func(r Repository) any { return r.Language },
	"size":        func(r Repository) any { return r.Size },
	"forkPolicy":  func(r Repository) any { return r.ForkPolicy },
	"createdAt":   func(r Repository) any { return r.CreatedOn },
	"updatedAt":   func(r Repository) any { return r.UpdatedOn },
	"owner":       func(r Repository) any { return r.Owner },
	"workspace":   func(r Repository) any { return r.Workspace },
	"project":     func(r Repository) any { return r.Project },
	"mainBranch": func(r Repository) any {
		if r.MainBranch == nil {
			return nil
		}
		return r.MainBranch.Name
	},
	"parent": func(r Repository) any {
		if r.Parent == nil {
			return nil
		}
		return r.Parent.FullName
	},
	"url":       func(r Repository) any { return r.Links.HTML() },
	"cloneUrls": func(r Repository) any { return r.CloneLinks() },
	"uuid":      func(r Repository) any { return r.UUID },
}

// PullRequestFields are the exportable fields for pull requests.
var PullRequestFields = Fields[PullRequest]{
	"id":          func(p PullRequest) any { return p.ID },
	"title":       func(p PullRequest) any { return p.Title },
	"body":        func(p PullRequest) any { return p.Description },
	"state":       func(p PullRequest) any { return p.State },
	"isDraft":     func(p PullRequest) any { return p.Draft },
	"author":      func(p PullRequest) any { return p.Author },
	"headRefName": func(p PullRequest) any { return p.Source.Branch.Name },
	"baseRefName": func(p PullRequest) any { return p.Destination.Branch.Name },
	"headRepository": func(p PullRequest) any {
		if p.Source.Repository == nil {
			return nil
		}
		return p.Source.Repository.FullName
	},
	"closeSourceBranch": func(p PullRequest) any { return p.CloseSourceBranch },
	"mergeCommit": func(p PullRequest) any {
		if p.MergeCommit == nil {
			return nil
		}
		return p.MergeCommit.Hash
	},
	"closedBy":     func(p PullRequest) any { return p.ClosedBy },
	"reason":       func(p PullRequest) any { return p.Reason },
	"commentCount": func(p PullRequest) any { return p.CommentCount },
	"taskCount":    func(p PullRequest) any { return p.TaskCount },
	"reviewers":    func(p PullRequest) any { return p.Reviewers },
	"participants": func(p PullRequest) any { return p.Participants },
	"createdAt":    func(p PullRequest) any { return p.CreatedOn },
	"updatedAt":    func(p PullRequest) any { return p.UpdatedOn },
	"url":          func(p PullRequest) any { return p.Links.HTML() },
}

// PipelineFields are the exportable fields for pipelines.
var PipelineFields = Fields[Pipeline]{
	"uuid":        func(p Pipeline) any { return p.UUID },
	"buildNumber": func(p Pipeline) any { return p.BuildNumber },
	"status":      func(p Pipeline) any { return p.State.Name },
	"result":      func(p Pipeline) any { return p.State.ResultName() },
	"refType":     func(p Pipeline) any { return p.Target.RefType },
	"refName":     func(p Pipeline) any { return p.Target.RefName },
	"commit": func(p Pipeline) any {
		if p.Target.Commit == nil {
			return nil
		}
		return p.Target.Commit.Hash
	},
	"creator": func(p Pipeline) any { return p.Creator },
	"trigger": func(p Pipeline) any {
		if p.Trigger == nil {
			return nil
		}
		return p.Trigger.Name
	},
	"createdAt":   func(p Pipeline) any { return p.CreatedOn },
	"completedAt": func(p Pipeline) any { return p.CompletedOn },
	"duration":    func(p Pipeline) any { return p.BuildSecondsUsed },
	"url":         func(p Pipeline) any { return p.Links.HTML() },
}

// WorkspaceFields are the exportable fields for workspaces.
var WorkspaceFields = Fields[Workspace]{
	"slug":       func(w Workspace) any { return w.Slug },
	"name":       func(w Workspace) any { return w.Name },
	"uuid":       func(w Workspace) any { return w.UUID },
	"isPrivate":  func(w Workspace) any { return w.IsPrivate },
	"isPersonal": func(w Workspace) any { return w.IsPersonal },
	"createdAt":  func(w Workspace) any { return w.CreatedOn },
	"url":        func(w Workspace) any { return w.Links.HTML() },
}

// ProjectFields are the exportable fields for projects.
var ProjectFields = Fields[Project]{
	"key":         func(p Project) any { return p.Key },
	"name":        func(p Project) any { return p.Name },
	"uuid":        func(p Project) any { return p.UUID },
	"description": func(p Project) any { return p.Description },
	"isPrivate":   func(p Project) any { return p.IsPrivate },
	"createdAt":   func(p Project) any { return p.CreatedOn },
	"updatedAt":   func(p Project) any { return p.UpdatedOn },
	"url":         func(p Project) any { return p.Links.HTML() },
}

// BranchFields are the exportable fields for branches/tags.
var BranchFields = Fields[Branch]{
	"name": func(b Branch) any { return b.Name },
	"target": func(b Branch) any {
		if b.Target == nil {
			return nil
		}
		return b.Target.Hash
	},
	"url": func(b Branch) any { return b.Links.HTML() },
}

// CommentFields are the exportable fields for comments.
var CommentFields = Fields[Comment]{
	"id":        func(c Comment) any { return c.ID },
	"body":      func(c Comment) any { return c.Content.Raw },
	"author":    func(c Comment) any { return c.User },
	"createdAt": func(c Comment) any { return c.CreatedOn },
	"updatedAt": func(c Comment) any { return c.UpdatedOn },
	"deleted":   func(c Comment) any { return c.Deleted },
	"inline":    func(c Comment) any { return c.Inline },
	"url":       func(c Comment) any { return c.Links.HTML() },
}
