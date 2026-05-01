package api

import (
	"net/url"
	"testing"
)

func TestParseIssueFilterIncludesTrackingState(t *testing.T) {
	t.Parallel()

	filter, projectKey := parseIssueFilter(url.Values{
		"project_key":    []string{"demo"},
		"tracking_state": []string{"reopened"},
	})

	if projectKey != "demo" {
		t.Fatalf("projectKey = %q, want demo", projectKey)
	}
	if filter.TrackingState == nil || *filter.TrackingState != "reopened" {
		t.Fatalf("tracking_state = %#v, want reopened", filter.TrackingState)
	}
}
