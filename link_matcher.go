package main

import (
	"regexp"
	"strings"
)

type LinkMatch struct {
	OriginalURL string
	TransformedURL string
	Type string
}

var (
	instagramRegex = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\binstagram\.com\/reel\/`)
	twitterRegex   = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\btwitter\.com\/`)
	xRegex         = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\bx\.com\/`)
	blueskyRegex   = regexp.MustCompile(`(?:https?:\/\/)?(?:www\.)?\bbsky\.app\/`)
)

func FindAndTransformLinks(text string) []LinkMatch {
	var matches []LinkMatch

	// Check for Instagram reel links
	if instagramRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL: text,
			TransformedURL: strings.Replace(text, "instagram.com", "kkinstagram.com", 1),
			Type: "instagram",
		})
	}

	// Check for Twitter links
	if twitterRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL: text,
			TransformedURL: strings.Replace(text, "twitter.com", "fxtwitter.com", 1),
			Type: "twitter",
		})
	}

	// Check for X.com links
	if xRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL: text,
			TransformedURL: strings.Replace(text, "x.com", "fixupx.com", 1),
			Type: "x",
		})
	}

	// Check for Bluesky links
	if blueskyRegex.MatchString(text) {
		matches = append(matches, LinkMatch{
			OriginalURL: text,
			TransformedURL: strings.Replace(text, "bsky.app", "fxbsky.app", 1),
			Type: "bluesky",
		})
	}

	return matches
}