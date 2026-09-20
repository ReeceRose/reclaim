package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"reclaim/internal/changelog"
	"reclaim/internal/version"
)

// releaseDTO is one changelog entry. Body is raw Markdown — the frontend
// renders the narrow subset release.sh emits rather than trusting HTML.
type releaseDTO struct {
	Tag     string `json:"tag"`
	Version string `json:"version"`
	Date    string `json:"date"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	Current bool   `json:"current"`
}

type releasesResponse struct {
	CurrentVersion string `json:"current_version"`
	RepoURL        string `json:"repo_url"`
	// WhatsNew is true when this build differs from the version whose notes the
	// user last acknowledged, and notes for it actually exist.
	WhatsNew bool         `json:"whats_new"`
	Releases []releaseDTO `json:"releases"`
}

// handleListReleases serves the changelog embedded in this binary. It makes no
// outbound request: the notes ship with the build, so they are available on an
// air-gapped install and can never describe a version other than the one
// running.
func (s *Server) handleListReleases(c *echo.Context) error {
	all := changelog.All()
	// Builds tag the version with and without a leading "v" depending on how
	// they were produced (CI strips it, `make build` did not always), so match
	// on the normalised form.
	current := strings.TrimPrefix(version.Version, "v")
	out := make([]releaseDTO, 0, len(all))
	for _, r := range all {
		out = append(out, releaseDTO{
			Tag:     r.Tag,
			Version: r.Version,
			Date:    r.Date,
			Body:    r.Body,
			URL:     r.URL,
			Current: version.IsRelease() && r.Version == current,
		})
	}

	resp := releasesResponse{
		CurrentVersion: version.Version,
		RepoURL:        version.RepoURL,
		Releases:       out,
	}

	// A dev build has no entry of its own, and an instance with no store (API
	// tests) has nowhere to record an acknowledgement.
	if version.IsRelease() && s.store != nil {
		if _, ok := changelog.For(version.Version); ok {
			lastSeen := s.store.Settings.LastSeenVersion(c.Request().Context())
			resp.WhatsNew = lastSeen != version.Version
		}
	}
	return c.JSON(http.StatusOK, resp)
}

// handleAckRelease records that the running build's notes have been seen, so
// the upgrade prompt does not reappear until the next release.
func (s *Server) handleAckRelease(c *echo.Context) error {
	if s.store == nil {
		return c.NoContent(http.StatusNoContent)
	}
	if err := s.store.Settings.SetLastSeenVersion(c.Request().Context(), version.Version); err != nil {
		return serverError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
