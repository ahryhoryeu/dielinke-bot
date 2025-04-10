package main

import (
	"testing"
)

func TestFindAndTransformLinks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []LinkMatch
	}{
		{
			name:  "Instagram reel link",
			input: "Check this out: instagram.com/reel/abc123",
			expected: []LinkMatch{
				{
					OriginalURL:    "Check this out: instagram.com/reel/abc123",
					TransformedURL: "Check this out: kkinstagram.com/reel/abc123",
					Type:          "instagram",
				},
			},
		},
		{
			name:  "Twitter link",
			input: "Found this on twitter.com/user/status/123",
			expected: []LinkMatch{
				{
					OriginalURL:    "Found this on twitter.com/user/status/123",
					TransformedURL: "Found this on fxtwitter.com/user/status/123",
					Type:          "twitter",
				},
			},
		},
		{
			name:  "X.com link",
			input: "Here's a link: x.com/user/status/456",
			expected: []LinkMatch{
				{
					OriginalURL:    "Here's a link: x.com/user/status/456",
					TransformedURL: "Here's a link: fixupx.com/user/status/456",
					Type:          "x",
				},
			},
		},
		{
			name:  "Bluesky link",
			input: "Check this: bsky.app/profile/user/post/789",
			expected: []LinkMatch{
				{
					OriginalURL:    "Check this: bsky.app/profile/user/post/789",
					TransformedURL: "Check this: fxbsky.app/profile/user/post/789",
					Type:          "bluesky",
				},
			},
		},
		{
			name:  "Multiple links",
			input: "instagram.com/reel/abc and twitter.com/user/123",
			expected: []LinkMatch{
				{
					OriginalURL:    "instagram.com/reel/abc and twitter.com/user/123",
					TransformedURL: "kkinstagram.com/reel/abc and twitter.com/user/123",
					Type:          "instagram",
				},
				{
					OriginalURL:    "instagram.com/reel/abc and twitter.com/user/123",
					TransformedURL: "instagram.com/reel/abc and fxtwitter.com/user/123",
					Type:          "twitter",
				},
			},
		},
		{
			name:     "No matching links",
			input:    "Just regular text",
			expected: []LinkMatch{},
		},
		{
			name:     "Similar but not matching domains",
			input:    "somethinginstagram.com or anythingx.com",
			expected: []LinkMatch{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindAndTransformLinks(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("FindAndTransformLinks() returned %d matches, want %d", len(got), len(tt.expected))
				return
			}

			for i, match := range got {
				if match.Type != tt.expected[i].Type {
					t.Errorf("FindAndTransformLinks() match %d type = %v, want %v", i, match.Type, tt.expected[i].Type)
				}
				if match.TransformedURL != tt.expected[i].TransformedURL {
					t.Errorf("FindAndTransformLinks() match %d transformed URL = %v, want %v", i, match.TransformedURL, tt.expected[i].TransformedURL)
				}
			}
		})
	}
}