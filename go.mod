module github.com/LukeSantossz/my-framework

// The `go` line is a hard floor: under GOTOOLCHAIN=local a build fails outright
// when the local toolchain is older, so it names the oldest release the code
// actually needs (any 1.26.x) rather than the newest one that happens to be
// installed. The `toolchain` line names the version a switching setup should
// fetch and is what actions/setup-go reads from this file, so CI stays pinned
// to an exact patch while a contributor on 1.26.0 is not forced to download one.
go 1.26

toolchain go1.26.7

require github.com/BurntSushi/toml v1.6.0
