package registry

import (
	"fmt"
	"strconv"
	"strings"
)

type UnmetDependency struct {
	PluginID          string `json:"plugin"`
	VersionConstraint string `json:"version_constraint"`
	Reason            string `json:"reason"`
}

func (u UnmetDependency) Error() string {
	return fmt.Sprintf("unmet dependency: plugin %q %s (%s)", u.PluginID, u.VersionConstraint, u.Reason)
}

type DependencyError struct {
	PluginID string            `json:"plugin_id"`
	Version  string            `json:"version"`
	Unmet    []UnmetDependency `json:"unmet"`
}

func (e *DependencyError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "plugin %q (version %s) has %d unmet dependencies:", e.PluginID, e.Version, len(e.Unmet))
	for _, u := range e.Unmet {
		sb.WriteString("\n  - ")
		sb.WriteString(u.Error())
	}
	return sb.String()
}

type InstalledPlugin struct {
	ID      string
	Version string
}

func ResolveDependencies(reg *PluginRegistry, pluginID, version string, installedPlugins []InstalledPlugin) error {
	entry, err := reg.Get(pluginID)
	if err != nil {
		return fmt.Errorf("resolve dependencies: %w", err)
	}

	if len(entry.Dependencies) == 0 {
		return nil
	}

	installed := make(map[string]string, len(installedPlugins))
	for _, p := range installedPlugins {
		installed[p.ID] = p.Version
	}

	var unmet []UnmetDependency
	for _, dep := range entry.Dependencies {
		installedVersion, ok := installed[dep.PluginID]
		if !ok {
			unmet = append(unmet, UnmetDependency{
				PluginID:          dep.PluginID,
				VersionConstraint: dep.VersionConstraint,
				Reason:            "not installed",
			})
			continue
		}

		if dep.VersionConstraint != "" {
			if !SatisfiesConstraint(installedVersion, dep.VersionConstraint) {
				unmet = append(unmet, UnmetDependency{
					PluginID:          dep.PluginID,
					VersionConstraint: dep.VersionConstraint,
					Reason:            fmt.Sprintf("installed version %s does not satisfy %s", installedVersion, dep.VersionConstraint),
				})
			}
		}
	}

	if len(unmet) > 0 {
		return &DependencyError{
			PluginID: pluginID,
			Version:  version,
			Unmet:    unmet,
		}
	}
	return nil
}

func SatisfiesConstraint(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}

	var op string
	var target string

	switch {
	case strings.HasPrefix(constraint, ">="):
		op, target = ">=", strings.TrimSpace(constraint[2:])
	case strings.HasPrefix(constraint, ">"):
		op, target = ">", strings.TrimSpace(constraint[1:])
	case strings.HasPrefix(constraint, "<="):
		op, target = "<=", strings.TrimSpace(constraint[2:])
	case strings.HasPrefix(constraint, "<"):
		op, target = "<", strings.TrimSpace(constraint[1:])
	case strings.HasPrefix(constraint, "!="):
		op, target = "!=", strings.TrimSpace(constraint[2:])
	case strings.HasPrefix(constraint, "=="):
		op, target = "=", strings.TrimSpace(constraint[2:])
	case strings.HasPrefix(constraint, "="):
		op, target = "=", strings.TrimSpace(constraint[1:])
	case strings.HasPrefix(constraint, "^"):
		op, target = "^", strings.TrimSpace(constraint[1:])
	case strings.HasPrefix(constraint, "~"):
		op, target = "~", strings.TrimSpace(constraint[1:])
	default:
		op, target = "=", constraint
	}

	cmp := CompareVersions(version, target)

	switch op {
	case ">=":
		return cmp >= 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case "<":
		return cmp < 0
	case "=":
		return cmp == 0
	case "!=":
		return cmp != 0
	case "^":
		if cmp < 0 {
			return false
		}
		vMajor, _, _ := parseSemver(version)
		tMajor, _, _ := parseSemver(target)
		return vMajor == tMajor
	case "~":
		if cmp < 0 {
			return false
		}
		vMajor, vMinor, _ := parseSemver(version)
		tMajor, tMinor, _ := parseSemver(target)
		return vMajor == tMajor && vMinor == tMinor
	}
	return false
}

func CompareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	maxLen := len(pa)
	if len(pb) > maxLen {
		maxLen = len(pb)
	}
	for i := range maxLen {
		var va, vb int
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) (major, minor, patch int) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) >= 1 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minor, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		patch, _ = strconv.Atoi(parts[2])
	}
	return
}
