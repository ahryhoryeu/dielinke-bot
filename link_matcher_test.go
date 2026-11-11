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
					OriginalURL:    "https://www.instagram.com/reel/abc123/",
					TransformedURL: "https://kkclip.com/reel/abc123/",
					Type:           "instagram",
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
					Type:           "twitter",
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
					Type:           "x",
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
					Type:           "bluesky",
				},
			},
		},
		{
			name:  "Multiple links",
			input: "instagram.com/reel/abc and twitter.com/user/123",
			expected: []LinkMatch{
				{
					OriginalURL:    "https://www.instagram.com/reel/abc/",
					TransformedURL: "https://kkclip.com/reel/abc/",
					Type:           "instagram",
				},
				{
					OriginalURL:    "instagram.com/reel/abc and twitter.com/user/123",
					TransformedURL: "instagram.com/reel/abc and fxtwitter.com/user/123",
					Type:           "twitter",
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

func TestFindInstagramReelURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		url   string
		ok    bool
	}{
		{
			name:  "plain URL",
			input: "instagram.com/reel/abc123",
			url:   "https://www.instagram.com/reel/abc123/",
			ok:    true,
		},
		{
			name:  "https URL with punctuation",
			input: "watch https://www.instagram.com/reel/abc123/?igsh=foo.",
			url:   "https://www.instagram.com/reel/abc123/",
			ok:    true,
		},
		{
			name:  "not reel",
			input: "https://www.instagram.com/p/abc123/",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, ok := FindInstagramReelURL(tt.input)
			if ok != tt.ok {
				t.Fatalf("FindInstagramReelURL() ok = %v, want %v", ok, tt.ok)
			}
			if url != tt.url {
				t.Fatalf("FindInstagramReelURL() url = %q, want %q", url, tt.url)
			}
		})
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("INSTAGRAM_REELS_MODE", "")
	t.Setenv("YTDLP_PATH", "")

	cfg := loadConfig()
	if cfg.InstagramReelsMode != "link" {
		t.Fatalf("InstagramReelsMode = %q, want link", cfg.InstagramReelsMode)
	}
	if cfg.YTDLPPath != "yt-dlp" {
		t.Fatalf("YTDLPPath = %q, want yt-dlp", cfg.YTDLPPath)
	}
}

func TestCanonicalReelURLs(t *testing.T) {
	for _, input := range []string{
		"http://instagram.com/reel/Ab_1-2?igsh=secret#fragment",
		"(https://www.instagram.com/reel/Ab_1-2/).",
		"[instagram.com/reel/Ab_1-2/];",
		"<https://INSTAGRAM.COM/reel/Ab_1-2/>!",
		"watch instagram.com/reel/Ab_1-2/, now",
		"https://evilinstagram.com/reel/no https://instagram.com/reel/Ab_1-2/",
	} {
		matches := FindAndTransformLinks(input)
		if len(matches) != 1 || matches[0].OriginalURL != "https://www.instagram.com/reel/Ab_1-2/" || matches[0].TransformedURL != "https://kkclip.com/reel/Ab_1-2/" {
			t.Errorf("input %q: %+v", input, matches)
		}
	}
	for _, input := range []string{
		"https://evilinstagram.com/reel/abc", "https://foo.instagram.com/reel/abc",
		"https://instagram.com.evil.test/reel/abc", "https://instagram.com@evil.test/reel/abc",
		"https://evil.test/instagram.com/reel/abc", "user@instagram.com/reel/abc",
		"https://evil.test/?u=https://instagram.com/reel/abc", "https://instagram.com:123/reel/abc",
		"ftp://instagram.com/reel/abc", "https://instagram.com/reel/", "https://instagram.com/reel/abc/extra",
		"https://instagram.com/reel/%2e%2e/", "https://instagram.com/reel/abc%2Fdef",
		"https://instagram.com/p/abc", "https://instagram.com/reel/a.b",
	} {
		if url, ok := FindInstagramReelURL(input); ok {
			t.Errorf("accepted %q as %q", input, url)
		}
	}
}
