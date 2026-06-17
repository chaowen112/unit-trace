package matching

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"unittrace/model"
)

// Candidate represents a tracked unit + latest snapshot data used for fuzzy matching.
type Candidate struct {
	TrackedUnitID   int64
	ProjectName     *string
	District        *string
	PropertyType    *string
	AddressText     *string
	Bedrooms        *int
	Bathrooms       *int
	FloorArea       *float64
	FloorLevelText  *string
	AgentName       *string
	Title           *string
	DescriptionText *string
	AskingPrice     *int64
	ImageURLs       []string
}

// MatchRequest holds the incoming listing data to match against known units.
type MatchRequest struct {
	Source          string
	Title           string
	ProjectName     string
	District        string
	PropertyType    string
	Bedrooms        int
	Bathrooms       int
	FloorArea       float64
	FloorLevelText  string
	AgentName       string
	AddressText     string
	AskingPrice     int64
	DescriptionText string
	ImageURLs       []string
}

// MatchResult holds the outcome of a match attempt.
type MatchResult struct {
	TrackedUnitID int64
	Score         int
	Status        string  // "seen_before", "likely_relisted", "almost_certain_same_unit", "possible_duplicate"
	Reasons       []string
	IsRelist      bool
}

// StoreReader is the interface the matching engine needs from the store.
type StoreReader interface {
	FindByListingID(ctx context.Context, source, listingID string) (*model.TrackedUnit, *model.ListingSnapshot, error)
	FindByCanonicalURL(ctx context.Context, source, canonicalURL string) (*model.TrackedUnit, *model.ListingSnapshot, error)
	GetFuzzyMatchCandidates(ctx context.Context, district *string, propertyType *string, bedrooms *int) ([]Candidate, error)
}

// Engine performs listing matching logic.
type Engine struct {
	store StoreReader
}

// NewEngine creates a new matching engine.
func NewEngine(store StoreReader) *Engine {
	return &Engine{store: store}
}

// ExactMatch attempts to find an exact match by listing ID or canonical URL.
func (e *Engine) ExactMatch(ctx context.Context, source, listingID, listingURL string) (*MatchResult, error) {
	if listingID != "" {
		unit, _, err := e.store.FindByListingID(ctx, source, listingID)
		if err != nil {
			return nil, err
		}
		if unit != nil {
			return &MatchResult{
				TrackedUnitID: unit.ID,
				Score:         100,
				Status:        "seen_before",
				Reasons:       []string{"matched by listing_id"},
				IsRelist:      false,
			}, nil
		}
	}

	if listingURL != "" {
		unit, _, err := e.store.FindByCanonicalURL(ctx, source, listingURL)
		if err != nil {
			return nil, err
		}
		if unit != nil {
			return &MatchResult{
				TrackedUnitID: unit.ID,
				Score:         100,
				Status:        "seen_before",
				Reasons:       []string{"matched by canonical_url"},
				IsRelist:      false,
			}, nil
		}
	}

	return nil, nil
}

// FuzzyMatch attempts to find a fuzzy match for the given listing.
// Returns nil if no match scores >= 60.
func (e *Engine) FuzzyMatch(ctx context.Context, req *MatchRequest) (*MatchResult, error) {
	var district *string
	var propertyType *string
	var bedrooms *int

	if req.District != "" {
		d := req.District
		district = &d
	}
	if req.PropertyType != "" {
		pt := req.PropertyType
		propertyType = &pt
	}
	if req.Bedrooms > 0 {
		b := req.Bedrooms
		bedrooms = &b
	}

	candidates, err := e.store.GetFuzzyMatchCandidates(ctx, district, propertyType, bedrooms)
	if err != nil {
		return nil, err
	}

	bestScore := -1
	var bestCandidate *Candidate
	var bestReasons []string

	for i := range candidates {
		c := &candidates[i]
		score, reasons := scoreCandidate(req, c)
		if score > bestScore {
			bestScore = score
			bestCandidate = c
			bestReasons = reasons
		}
	}

	if bestScore < 60 || bestCandidate == nil {
		return nil, nil
	}

	status := statusFromScore(bestScore)
	isRelist := bestScore >= 75 && bestScore < 90

	return &MatchResult{
		TrackedUnitID: bestCandidate.TrackedUnitID,
		Score:         bestScore,
		Status:        status,
		Reasons:       bestReasons,
		IsRelist:      isRelist,
	}, nil
}

func statusFromScore(score int) string {
	switch {
	case score >= 90:
		return "almost_certain_same_unit"
	case score >= 75:
		return "likely_relisted"
	default:
		return "possible_duplicate"
	}
}

// scoreCandidate scores a candidate against an incoming listing.
//
// Design: core identifying fields are HARD REQUIREMENTS — all must match or the
// candidate is rejected outright. Only after passing do soft signals (description,
// images, price) add confidence. This prevents coincidental matches between
// different units that happen to share project/district/bedroom count.
//
// Scoring bands (unchanged from PRD):
//   >= 90 → almost_certain_same_unit (auto-merge)
//   75-89 → likely_relisted (auto-merge)
//   60-74 → possible_duplicate (shown, not merged)
//      <60 → not a match
func scoreCandidate(req *MatchRequest, c *Candidate) (int, []string) {
	// --- Hard requirements ---
	// All must pass. These fields cannot change in a genuine relist.

	// property_type must match
	if c.PropertyType != nil && req.PropertyType != "" {
		if !strings.EqualFold(strings.TrimSpace(req.PropertyType), strings.TrimSpace(*c.PropertyType)) {
			return 0, nil
		}
	}

	// district must match
	if c.District != nil && req.District != "" {
		if !strings.EqualFold(strings.TrimSpace(req.District), strings.TrimSpace(*c.District)) {
			return 0, nil
		}
	}

	// bedrooms must match
	if c.Bedrooms != nil && req.Bedrooms > 0 {
		if req.Bedrooms != *c.Bedrooms {
			return 0, nil
		}
	}

	// bathrooms must match
	if c.Bathrooms != nil && req.Bathrooms > 0 {
		if req.Bathrooms != *c.Bathrooms {
			return 0, nil
		}
	}

	// floor_area must be within 3%
	if c.FloorArea != nil && *c.FloorArea > 0 && req.FloorArea > 0 {
		if math.Abs(req.FloorArea-*c.FloorArea) / *c.FloorArea > 0.03 {
			return 0, nil
		}
	}

	// title must be similar (trigram >= 0.4) when both are present
	if c.Title != nil && *c.Title != "" && req.Title != "" {
		if trigramSimilarity(req.Title, *c.Title) < 0.4 {
			return 0, nil
		}
	}

	// agent_name must match when both are present
	if c.AgentName != nil && *c.AgentName != "" && req.AgentName != "" {
		if !strings.EqualFold(strings.TrimSpace(req.AgentName), strings.TrimSpace(*c.AgentName)) {
			return 0, nil
		}
	}

	// address_text must be similar (trigram >= 0.4) when both are present
	if c.AddressText != nil && *c.AddressText != "" && req.AddressText != "" {
		if trigramSimilarity(req.AddressText, *c.AddressText) < 0.4 {
			return 0, nil
		}
	}

	// --- Passed all hard requirements ---
	// Base score 60. Soft signals push it higher.
	score := 60
	reasons := []string{"core_fields_match"}

	// description trigram similarity >= 0.4: +15
	if c.DescriptionText != nil && *c.DescriptionText != "" && req.DescriptionText != "" {
		if trigramSimilarity(req.DescriptionText, *c.DescriptionText) >= 0.4 {
			score += 15
			reasons = append(reasons, "description_match")
		}
	}

	// image URL exact match (any shared URL): +40
	if len(req.ImageURLs) > 0 && len(c.ImageURLs) > 0 {
		reqSet := make(map[string]struct{}, len(req.ImageURLs))
		for _, u := range req.ImageURLs {
			reqSet[u] = struct{}{}
		}
		for _, u := range c.ImageURLs {
			if _, ok := reqSet[u]; ok {
				score += 40
				reasons = append(reasons, "image_match")
				break
			}
		}
	}

	// psf within 10%: +10
	if c.FloorArea != nil && *c.FloorArea > 0 && req.FloorArea > 0 &&
		c.AskingPrice != nil && *c.AskingPrice > 0 && req.AskingPrice > 0 {
		reqPSF := float64(req.AskingPrice) / req.FloorArea
		cPSF := float64(*c.AskingPrice) / *c.FloorArea
		if math.Abs(reqPSF-cPSF)/cPSF <= 0.10 {
			score += 10
			reasons = append(reasons, "psf_within_10pct")
		}
	}

	// price within 15%: +5
	if c.AskingPrice != nil && *c.AskingPrice > 0 && req.AskingPrice > 0 {
		if math.Abs(float64(req.AskingPrice)-float64(*c.AskingPrice))/float64(*c.AskingPrice) <= 0.15 {
			score += 5
			reasons = append(reasons, "price_within_15pct")
		}
	}

	return score, reasons
}

// trigramSimilarity computes the Jaccard similarity of trigram sets for two strings.
func trigramSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	triA := buildTrigrams(strings.ToLower(a))
	triB := buildTrigrams(strings.ToLower(b))
	if len(triA) == 0 || len(triB) == 0 {
		return 0
	}

	setA := make(map[string]struct{}, len(triA))
	for _, t := range triA {
		setA[t] = struct{}{}
	}

	intersection := 0
	setB := make(map[string]struct{}, len(triB))
	for _, t := range triB {
		if _, ok := setA[t]; ok {
			intersection++
		}
		setB[t] = struct{}{}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// buildTrigrams returns a sorted list of trigrams for a string (padded with spaces).
func buildTrigrams(s string) []string {
	padded := "  " + s + "  "
	runes := []rune(padded)
	if utf8.RuneCountInString(padded) < 3 {
		return nil
	}

	trigrams := make([]string, 0, len(runes)-2)
	for i := 0; i <= len(runes)-3; i++ {
		trigrams = append(trigrams, string(runes[i:i+3]))
	}
	sort.Strings(trigrams)
	return trigrams
}
