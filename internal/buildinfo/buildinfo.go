// Package buildinfo provides build-time information shared by all EJQuick
// executables.
package buildinfo

// Version is the unified EJQuick product version. Release builds override it
// from the Git tag by using the Go linker's -X option.
var Version = "dev"
