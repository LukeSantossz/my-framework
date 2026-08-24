// Package version carries the build's identity.
package version

// Version is overridden at release time with -ldflags. The default names an
// unreleased build so a lock file written by one is never mistaken for a
// released adoption.
var Version = "0.0.0-dev"
