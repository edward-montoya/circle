// Package domaindoc finds, indexes and checks a repository's domain model: its
// DOMAIN.md files, its ADRs and its architecture documents.
package domaindoc

import (
	"path"
	"strings"
)

// FileName is the per-domain document. A repository may have many — one per
// bounded context — and each covers the directory it sits in.
const FileName = "DOMAIN.md"

// IndexName is the generated index Circle keeps under .circle/.
const IndexName = "DOMAIN-INDEX.md"

// IsDomainDoc reports whether a repo-relative path is part of the domain model:
// a DOMAIN.md, an ADR, or an architecture document.
func IsDomainDoc(rel string) bool {
	rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
	base := path.Base(rel)
	switch {
	case base == FileName:
		return true
	case strings.EqualFold(base, "ARCHITECTURE.md"):
		return true
	case strings.HasSuffix(base, ".md") && inDir(rel, "adr"):
		return true
	case strings.HasSuffix(base, ".md") && inDir(rel, "architecture"):
		return true
	}
	return false
}

// inDir reports whether any directory component of rel is name.
func inDir(rel, name string) bool {
	parts := strings.Split(path.Dir(rel), "/")
	for _, p := range parts {
		if p == name {
			return true
		}
	}
	return false
}
