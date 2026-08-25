package bitbucket

import "encoding/json"

// repositoryJSON mirrors Repository but captures links.clone (an array,
// unlike the other link entries which are objects).
type repositoryJSON Repository

// UnmarshalJSON parses a repository, extracting clone links.
func (r *Repository) UnmarshalJSON(data []byte) error {
	var aux struct {
		repositoryJSON
		Links struct {
			HTML  Link   `json:"html"`
			Clone []Link `json:"clone"`
		} `json:"links"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = Repository(aux.repositoryJSON)
	r.Links = Links{"html": aux.Links.HTML}
	r.clone = aux.Links.Clone
	return nil
}

// MarshalJSON emits the repository including clone links.
func (r Repository) MarshalJSON() ([]byte, error) {
	type out struct {
		repositoryJSON
		Links map[string]any `json:"links,omitempty"`
	}
	o := out{repositoryJSON: repositoryJSON(r)}
	o.repositoryJSON.Links = nil
	links := map[string]any{}
	if h := r.Links.HTML(); h != "" {
		links["html"] = Link{Href: h}
	}
	if len(r.clone) > 0 {
		links["clone"] = r.clone
	}
	if len(links) > 0 {
		o.Links = links
	}
	return json.Marshal(o)
}
