package pipeline

import (
	"encoding/json"

	"github.com/uehatsu/bb/internal/bitbucket"
)

type bitbucketTarget = bitbucket.PipelineTarget

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
