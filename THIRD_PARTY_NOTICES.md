# Third-Party Notices

`sgh-cli` is distributed under the [MIT License](LICENSE). It links the Go
modules listed below, each under its own license. Every license here is
permissive and compatible with MIT; none of them impose copyleft obligations on
this project or on binaries built from it.

Generated from `go list -deps ./...`, so this covers what is actually linked
into the binary — direct and transitive.

## Apache License 2.0

- `github.com/spf13/cobra` — Copyright 2013–2023 The Cobra Authors
- `github.com/inconshreveable/mousetrap` — Copyright 2022 Alan Shreve (linked on
  Windows builds only)

Full text: https://www.apache.org/licenses/LICENSE-2.0

## BSD 3-Clause License

- `github.com/atotto/clipboard`
- `github.com/spf13/pflag`
- `golang.org/x/oauth2`
- `golang.org/x/sys`
- `golang.org/x/term`
- `golang.org/x/text`

## MIT License

- `github.com/MakeNowJust/heredoc`
- `github.com/aymanbagabas/go-osc52/v2`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/colorprofile`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/x/ansi`
- `github.com/charmbracelet/x/cellbuf`
- `github.com/charmbracelet/x/term`
- `github.com/clipperhouse/displaywidth`
- `github.com/clipperhouse/uax29/v2`
- `github.com/k0kubun/go-ansi`
- `github.com/lithammer/fuzzysearch`
- `github.com/lucasb-eyer/go-colorful`
- `github.com/mattn/go-colorable`
- `github.com/mattn/go-isatty`
- `github.com/mattn/go-runewidth`
- `github.com/mitchellh/colorstring`
- `github.com/muesli/ansi`
- `github.com/muesli/cancelreader`
- `github.com/muesli/termenv`
- `github.com/rivo/uniseg`
- `github.com/rs/zerolog`
- `github.com/sahilm/fuzzy`
- `github.com/schollz/progressbar/v3`
- `github.com/shurcooL/githubv4`
- `github.com/shurcooL/graphql`
- `github.com/xo/terminfo`

## Test-only dependencies

Used by the test suite, not linked into released binaries:

- `github.com/stretchr/testify` — MIT
- `github.com/davecgh/go-spew` — ISC
- `github.com/pmezard/go-difflib` — BSD 3-Clause
- `gopkg.in/yaml.v3` — MIT and Apache-2.0
