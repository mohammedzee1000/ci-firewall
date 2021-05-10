package version

/*
===================
=    IMPORTANT    =
===================
This package is solely for versioning information when releasing cli..
Changing these values will change the versioning information when releasing cli.
*/

var (
	// VERSION  is version number that will be displayed when running ./odo version
	VERSION = "main"

	// GITCOMMIT is hash of the commit that will be displayed when running ./ci-firewall version
	// this will be overwritten when running  build like this: go build -ldflags="-X github.com/mohammedzee1000/ci-firewall/cmd.GITCOMMIT=$(GITCOMMIT)"
	// HEAD is default indicating that this was not set during build
	GITCOMMIT = "HEAD"
)
