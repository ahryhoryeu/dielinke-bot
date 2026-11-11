package main

import (
	"net/url"
	"regexp"
	"strings"
)

type LinkMatch struct {
	OriginalURL    string
	TransformedURL string
	Type           string
}

var (
	urlTokenRegex = regexp.MustCompile(`[^\s<>"'(){}\[\]]+`)
	reelPathRegex = regexp.MustCompile(`^/reel/([A-Za-z0-9_-]+)/?$`)
	twitterRegex  = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\btwitter\.com\/`)
	xRegex        = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\bx\.com\/`)
	blueskyRegex  = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\bbsky\.app\/`)
)

// Parse whole tokens so a lookalike host or a URL nested in another URL
// cannot be mistaken for an Instagram link.
func FindInstagramReelURL(text string) (string, bool) {
	for _, token := range urlTokenRegex.FindAllString(text, -1) {
		token = strings.TrimRight(token, ".,!?;:")
		if !strings.Contains(token, "://") {
			token = "https://" + token
		}
		u, err := url.Parse(token)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			continue
		}
		host := strings.ToLower(u.Host)
		if host != "instagram.com" && host != "www.instagram.com" {
			continue
		}
		match := reelPathRegex.FindStringSubmatch(u.EscapedPath())
		if match != nil {
			return "https://www.instagram.com/reel/" + match[1] + "/", true
		}
	}
	return "", false
}

func FindAndTransformLinks(text string) []LinkMatch {
	var matches []LinkMatch

	// Check for Instagram reel links
	if instagramURL, ok := FindInstagramReelURL(text); ok {
		matches = append(matches, LinkMatch{
			OriginalURL:    instagramURL,
			TransformedURL: strings.Replace(instagramURL, "www.instagram.com", "kkclip.com", 1),
			Type:           "instagram",
		})
	}

	// Check for Twitter links
	if twitterRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL:    text,
			TransformedURL: strings.Replace(text, "twitter.com", "fxtwitter.com", 1),
			Type:           "twitter",
		})
	}

	// Check for X.com links
	if xRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL:    text,
			TransformedURL: strings.Replace(text, "x.com", "fixupx.com", 1),
			Type:           "x",
		})
	}

	// Check for Bluesky links
	if blueskyRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL:    text,
			TransformedURL: strings.Replace(text, "bsky.app", "fxbsky.app", 1),
			Type:           "bluesky",
		})
	}

	return matches
}
