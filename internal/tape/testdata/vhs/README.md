Every `.tape` in Charm VHS at commit `24fa2254a9806091e6ee6a980e9f3bcfe0a9ba53`,
copied unchanged under its upstream path and covered by the included MIT license.
`MANIFEST` records each file's git blob SHA-1; a test verifies that the files are
exactly upstream's.

They are parsed in tests, not executed: some need external applications, fonts, or
platform-specific shells. See `corpus_test.go` for the tapes ntrecord rejects and
why. End-to-end tests use local fixtures.
