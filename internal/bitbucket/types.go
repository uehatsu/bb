// Package bitbucket defines lenient Go representations of Bitbucket Cloud
// API objects. Only fields the CLI uses are declared; unknown fields are
// ignored so that API additions never break decoding.
package bitbucket

import "time"

// Links is the common `links` map: {"html": {"href": "..."}, ...}.
type Links map[string]Link

// Link is a single hyperlink entry.
type Link struct {
	Href string `json:"href"`
	Name string `json:"name,omitempty"`
}

// HTML returns the html link href, if present.
func (l Links) HTML() string { return l["html"].Href }

// Account is a user or team reference.
type Account struct {
	Type        string `json:"type,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Nickname    string `json:"nickname,omitempty"`
	Links       Links  `json:"links,omitempty"`
}

// Name returns the best human-readable handle.
func (a Account) Name() string {
	if a.Nickname != "" {
		return a.Nickname
	}
	return a.DisplayName
}

// User is the authenticated user (GET /user).
type User struct {
	Account
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

// Workspace is a Bitbucket workspace.
type Workspace struct {
	Type       string    `json:"type,omitempty"`
	UUID       string    `json:"uuid,omitempty"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	IsPrivate  bool      `json:"is_private"`
	IsPersonal bool      `json:"is_personal"`
	CreatedOn  time.Time `json:"created_on,omitempty"`
	Links      Links     `json:"links,omitempty"`
}

// WorkspaceMembership is an entry from /user/workspaces or /workspaces/{ws}/members.
type WorkspaceMembership struct {
	Permission string    `json:"permission,omitempty"`
	User       Account   `json:"user"`
	Workspace  Workspace `json:"workspace"`
}

// Project groups repositories in a workspace.
type Project struct {
	Type        string    `json:"type,omitempty"`
	UUID        string    `json:"uuid,omitempty"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsPrivate   bool      `json:"is_private"`
	CreatedOn   time.Time `json:"created_on,omitempty"`
	UpdatedOn   time.Time `json:"updated_on,omitempty"`
	Links       Links     `json:"links,omitempty"`
}

// Repository is a Bitbucket repository.
type Repository struct {
	Type        string      `json:"type,omitempty"`
	UUID        string      `json:"uuid,omitempty"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	FullName    string      `json:"full_name"`
	Description string      `json:"description,omitempty"`
	IsPrivate   bool        `json:"is_private"`
	SCM         string      `json:"scm,omitempty"`
	Language    string      `json:"language,omitempty"`
	Size        int64       `json:"size,omitempty"`
	ForkPolicy  string      `json:"fork_policy,omitempty"`
	CreatedOn   time.Time   `json:"created_on,omitempty"`
	UpdatedOn   time.Time   `json:"updated_on,omitempty"`
	Owner       *Account    `json:"owner,omitempty"`
	Workspace   *Workspace  `json:"workspace,omitempty"`
	Project     *Project    `json:"project,omitempty"`
	MainBranch  *Branch     `json:"mainbranch,omitempty"`
	Parent      *Repository `json:"parent,omitempty"`
	Links       Links       `json:"links,omitempty"`
	clone       []Link
}

// CloneURL returns the clone link for the given protocol ("https"|"ssh").
func (r Repository) CloneURL(protocol string) string {
	for _, l := range r.CloneLinks() {
		if l.Name == protocol {
			return l.Href
		}
	}
	return ""
}

// CloneLinks returns links.clone entries.
func (r Repository) CloneLinks() []Link {
	// links.clone is an array, so it is not representable in Links; it is
	// parsed separately via RepositoryClone.
	return r.clone
}

// Commit is a git commit summary.
type Commit struct {
	Type    string    `json:"type,omitempty"`
	Hash    string    `json:"hash"`
	Message string    `json:"message,omitempty"`
	Date    time.Time `json:"date,omitempty"`
	Author  struct {
		Raw  string   `json:"raw,omitempty"`
		User *Account `json:"user,omitempty"`
	} `json:"author,omitempty"`
	Links Links `json:"links,omitempty"`
}

// ShortHash returns the first 7 characters of the hash.
func (c Commit) ShortHash() string {
	if len(c.Hash) > 7 {
		return c.Hash[:7]
	}
	return c.Hash
}

// Branch/Tag reference.
type Branch struct {
	Type   string  `json:"type,omitempty"`
	Name   string  `json:"name"`
	Target *Commit `json:"target,omitempty"`
	Links  Links   `json:"links,omitempty"`
}

// Ref is an alias used for tags.
type Ref = Branch

// PullRequestEndpoint is source/destination of a PR.
type PullRequestEndpoint struct {
	Branch     Branch      `json:"branch"`
	Commit     *Commit     `json:"commit,omitempty"`
	Repository *Repository `json:"repository,omitempty"`
}

// Participant is a reviewer/participant on a PR.
type Participant struct {
	Type           string    `json:"type,omitempty"`
	User           Account   `json:"user"`
	Role           string    `json:"role,omitempty"` // PARTICIPANT | REVIEWER
	Approved       bool      `json:"approved"`
	State          string    `json:"state,omitempty"` // approved | changes_requested | null
	ParticipatedOn time.Time `json:"participated_on,omitempty"`
}

// PullRequest is a Bitbucket pull request.
type PullRequest struct {
	Type              string              `json:"type,omitempty"`
	ID                int                 `json:"id"`
	Title             string              `json:"title"`
	Description       string              `json:"description,omitempty"`
	State             string              `json:"state"` // OPEN | MERGED | DECLINED | SUPERSEDED
	Draft             bool                `json:"draft"`
	Author            Account             `json:"author"`
	Source            PullRequestEndpoint `json:"source"`
	Destination       PullRequestEndpoint `json:"destination"`
	CloseSourceBranch bool                `json:"close_source_branch"`
	MergeCommit       *Commit             `json:"merge_commit,omitempty"`
	ClosedBy          *Account            `json:"closed_by,omitempty"`
	Reason            string              `json:"reason,omitempty"`
	CommentCount      int                 `json:"comment_count"`
	TaskCount         int                 `json:"task_count"`
	Reviewers         []Account           `json:"reviewers,omitempty"`
	Participants      []Participant       `json:"participants,omitempty"`
	CreatedOn         time.Time           `json:"created_on,omitempty"`
	UpdatedOn         time.Time           `json:"updated_on,omitempty"`
	Summary           *Rendered           `json:"summary,omitempty"`
	Links             Links               `json:"links,omitempty"`
}

// Rendered is a markup content object {raw, markup, html}.
type Rendered struct {
	Raw    string `json:"raw"`
	Markup string `json:"markup,omitempty"`
	HTML   string `json:"html,omitempty"`
}

// Comment is a PR/commit comment.
type Comment struct {
	Type      string    `json:"type,omitempty"`
	ID        int       `json:"id"`
	Content   Rendered  `json:"content"`
	User      Account   `json:"user"`
	Deleted   bool      `json:"deleted"`
	Pending   bool      `json:"pending,omitempty"`
	CreatedOn time.Time `json:"created_on,omitempty"`
	UpdatedOn time.Time `json:"updated_on,omitempty"`
	Inline    *struct {
		Path string `json:"path"`
		From *int   `json:"from,omitempty"`
		To   *int   `json:"to,omitempty"`
	} `json:"inline,omitempty"`
	Parent *struct {
		ID int `json:"id"`
	} `json:"parent,omitempty"`
	Links Links `json:"links,omitempty"`
}

// CommitStatus is a build status attached to a commit / PR.
type CommitStatus struct {
	Type        string    `json:"type,omitempty"`
	Key         string    `json:"key"`
	Name        string    `json:"name,omitempty"`
	State       string    `json:"state"` // SUCCESSFUL | FAILED | INPROGRESS | STOPPED
	Description string    `json:"description,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedOn   time.Time `json:"created_on,omitempty"`
	UpdatedOn   time.Time `json:"updated_on,omitempty"`
}

// DiffStat is one entry of a diffstat.
type DiffStat struct {
	Type         string `json:"type,omitempty"`
	Status       string `json:"status"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	Old          *struct {
		Path string `json:"path"`
	} `json:"old,omitempty"`
	New *struct {
		Path string `json:"path"`
	} `json:"new,omitempty"`
}

// Path returns the most relevant path of the diffstat entry.
func (d DiffStat) Path() string {
	if d.New != nil {
		return d.New.Path
	}
	if d.Old != nil {
		return d.Old.Path
	}
	return ""
}

// PipelineState describes pipeline progress.
type PipelineState struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name"` // PENDING | IN_PROGRESS | COMPLETED
	Stage *struct {
		Name string `json:"name"`
	} `json:"stage,omitempty"`
	Result *struct {
		Name string `json:"name"` // SUCCESSFUL | FAILED | ERROR | STOPPED | EXPIRED
	} `json:"result,omitempty"`
}

// ResultName returns the result name or "".
func (s PipelineState) ResultName() string {
	if s.Result != nil {
		return s.Result.Name
	}
	return ""
}

// PipelineTarget is the trigger target.
type PipelineTarget struct {
	Type     string  `json:"type"`
	RefType  string  `json:"ref_type,omitempty"`
	RefName  string  `json:"ref_name,omitempty"`
	Commit   *Commit `json:"commit,omitempty"`
	Selector *struct {
		Type    string `json:"type"`
		Pattern string `json:"pattern"`
	} `json:"selector,omitempty"`
}

// Pipeline is a pipeline run.
type Pipeline struct {
	Type        string         `json:"type,omitempty"`
	UUID        string         `json:"uuid"`
	BuildNumber int            `json:"build_number"`
	State       PipelineState  `json:"state"`
	Target      PipelineTarget `json:"target"`
	Creator     *Account       `json:"creator,omitempty"`
	Trigger     *struct {
		Name string `json:"name"`
	} `json:"trigger,omitempty"`
	CreatedOn        time.Time `json:"created_on,omitempty"`
	CompletedOn      time.Time `json:"completed_on,omitempty"`
	BuildSecondsUsed int       `json:"build_seconds_used,omitempty"`
	Links            Links     `json:"links,omitempty"`
}

// PipelineStep is a single step within a pipeline.
type PipelineStep struct {
	Type        string        `json:"type,omitempty"`
	UUID        string        `json:"uuid"`
	Name        string        `json:"name,omitempty"`
	State       PipelineState `json:"state"`
	StartedOn   time.Time     `json:"started_on,omitempty"`
	CompletedOn time.Time     `json:"completed_on,omitempty"`
	DurationSec int           `json:"duration_in_seconds,omitempty"`
}
