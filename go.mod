module github.com/willabides/pgdevserver

go 1.24.0

require (
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/adrg/xdg v0.5.3
	github.com/alecthomas/kong v1.8.1
	github.com/jackc/pgx/v5 v5.7.2
	github.com/mholt/archives v0.1.0
	github.com/rogpeppe/go-internal v1.13.1
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/STARRY-S/zip v0.2.1 // indirect
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/bodgit/plumbing v1.3.0 // indirect
	github.com/bodgit/sevenzip v1.6.0 // indirect
	github.com/bodgit/windows v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/nwaples/rardecode/v2 v2.1.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sorairolake/lzip-go v0.3.5 // indirect
	github.com/therootcompany/xz v1.0.1 // indirect
	github.com/ulikunitz/xz v0.5.12 // indirect
	go4.org v0.0.0-20230225012048-214862532bf5 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// remove after https://github.com/mholt/archives/issues/17 is fixed
replace github.com/mholt/archives => github.com/willabides/archives v0.1.1-0.20250216175030-bbb6eb95ca5c
