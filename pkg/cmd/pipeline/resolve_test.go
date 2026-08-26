package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

// fakePipelines mimics the live endpoint: q= is ignored, sort=-created_on
// and page/pagelen work, build numbers may have gaps.
func fakePipelines(t *testing.T, h *testutil.Harness, numbers []int) *[]string {
	t.Helper()
	var requests []string
	h.Handle("/repositories/acme/widgets/pipelines", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RawQuery)
		q := r.URL.Query()
		items := append([]int(nil), numbers...) // oldest first by default
		if q.Get("sort") == "-created_on" {
			for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
				items[i], items[j] = items[j], items[i]
			}
		}
		pagelen, _ := strconv.Atoi(q.Get("pagelen"))
		if pagelen <= 0 {
			pagelen = 10
		}
		page, _ := strconv.Atoi(q.Get("page"))
		if page <= 0 {
			page = 1
		}
		start := (page - 1) * pagelen
		end := start + pagelen
		if start > len(items) {
			start = len(items)
		}
		if end > len(items) {
			end = len(items)
		}
		var vals []string
		for _, n := range items[start:end] {
			vals = append(vals, fmt.Sprintf(`{"uuid":"{p%d}","build_number":%d,"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}},"target":{"ref_name":"main"}}`, n, n))
		}
		next := ""
		if end < len(items) {
			next = fmt.Sprintf(`,"next":"%s/2.0/repositories/acme/widgets/pipelines?page=%d&pagelen=%d&sort=-created_on"`, h.Server.URL, page+1, pagelen)
		}
		w.Write([]byte(`{"values":[` + strings.Join(vals, ",") + `]` + next + `}`))
	})
	return &requests
}

func TestFindByBuildNumber(t *testing.T) {
	h := testutil.NewHarness(t)
	// 1..380 with a gap at 200..209 (deleted builds)
	var nums []int
	for i := 1; i <= 380; i++ {
		if i >= 200 && i <= 209 {
			continue
		}
		nums = append(nums, i)
	}
	reqs := fakePipelines(t, h, nums)
	client, _ := h.Factory.APIClient()
	repo := h.Factory.BaseRepo
	r, _ := repo()

	cases := []struct {
		n        int
		maxCalls int
	}{
		{380, 1}, // newest, first page
		{379, 1},
		{300, 1},
		{150, 2}, // predicted page hit
		{1, 2},
		{195, 4}, // gap shifts the prediction; fallback scan
	}
	for _, tc := range cases {
		*reqs = nil
		p, err := findByBuildNumber(t.Context(), client, r, tc.n)
		if err != nil || p == nil || p.BuildNumber != tc.n {
			t.Errorf("#%d: got %v err=%v", tc.n, p, err)
			continue
		}
		if len(*reqs) > tc.maxCalls {
			t.Errorf("#%d: %d requests, want <= %d: %v", tc.n, len(*reqs), tc.maxCalls, *reqs)
		}
		for _, q := range *reqs {
			if strings.Contains(q, "q=") {
				t.Errorf("#%d: must not rely on the ignored q= filter: %s", tc.n, q)
			}
		}
	}
	for _, missing := range []int{205, 381, 9999} {
		if _, err := findByBuildNumber(t.Context(), client, r, missing); err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("#%d: expected not found, got %v", missing, err)
		}
	}
}

func TestViewAndWatchUseBuildNumber(t *testing.T) {
	h := testutil.NewHarness(t)
	fakePipelines(t, h, []int{1, 2, 3, 4, 5})
	h.JSON("GET", "/repositories/acme/widgets/pipelines/{p4}/steps", 200, `{"values":[]}`)
	v := NewCmdView(h.Factory)
	v.SetArgs([]string{"4", "--json", "buildNumber"})
	if err := v.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(h.Out.Bytes(), &got)
	if got["buildNumber"] != float64(4) {
		t.Errorf("view 4 resolved to %v (regression: used to return the oldest build)", got)
	}
	h.JSON("GET", "/repositories/acme/widgets/pipelines/{p3}", 200, `{"uuid":"{p3}","build_number":3,"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}}}`)
	wcmd := NewCmdWatch(h.Factory)
	wcmd.SetArgs([]string{"3"})
	if err := wcmd.Execute(); err != nil || !strings.Contains(h.ErrOut.String(), "Pipeline #3 succeeded") {
		t.Errorf("watch 3: err=%v stderr=%s", err, h.ErrOut.String())
	}
}
