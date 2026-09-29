# my-gh-classroom

`my-gh-classroom` is a lightweight command-line utility for managing
GitHub organizations used as programming classrooms. The compiled binary
is named `mgc`. 

It is designed for a simple model:

- one GitHub organization per classroom/course offering;
- one private repository per student;
- assignments stored as directories inside the student’s repository;
- a GitHub team for instructors and TAs; and
- a public `course-info` repository containing shared Git/GitHub and
  submission instructions.

The utility uses the official GitHub CLI (`gh`) for authentication and
GitHub API access. It does not store GitHub credentials.

> **Safety:** commands that change configuration or GitHub are **dry-run
> by default**. Pass `--apply` (or `-a`) explicitly to perform a
> mutation.

## Features

- Manage multiple classrooms from one installation.
- Give each classroom a short local alias such as `cs472` or `cs281`.
- Select a classroom globally with `--classroom` or `-c`.
- Configure a default classroom for commands where no classroom is
  specified.
- Create, list, delete, select, and verify classroom configurations.
- Manage instructor/TA grader teams.
- Create individual private student repositories.
- Grant student and grader-team repository access.
- Initialize student repositories with a README linking to the
  classroom’s `course-info` repository.
- Store searchable student metadata using GitHub repository custom
  properties.
- Find students by name, GitHub ID, or repository name.
- Inspect student repository permissions and pending invitations.
- Report pending invitations with the URL each student can use to
  accept, for the whole class or one student (`student invites`).
- Generate short, ready-to-paste messages (`--message`) for students
  and TAs, signed by the classroom's instructors.
- Show TAs and graders who have not accepted their organization
  invitation, with the accept URL (`team invites`, `team list`).
- Import students from any CSV roster (`classroom import`), as a
  repeatable upsert with an on-screen and saved report.
- Safely skip existing student repositories rather than overwrite
  student work, while repairing any missing access or metadata.

## Prerequisites

You need:

- Go 1.23 or newer to build the utility
- GitHub CLI (`gh`)
- a GitHub account
- one or more GitHub organizations that you administer

Once `mgc` is built, Go itself is not required to run the executable.
The GitHub CLI remains a runtime prerequisite because `mgc` deliberately
delegates authentication and API access to `gh`.

## GitHub Education

Educators should consider applying for **GitHub Education** before
setting up course organizations. Verified teachers can receive GitHub
education benefits that are useful for courses with private student
repositories.

GitHub also offers the **GitHub Campus Program** for eligible
institutions.

Current information:

- GitHub Education for teachers:
  https://docs.github.com/en/education/about-github-education/github-education-for-teachers
- Apply as a teacher:
  https://docs.github.com/en/education/about-github-education/github-education-for-teachers/apply-to-github-education-as-a-teacher
- GitHub Campus Program:
  https://docs.github.com/en/education/about-github-education/use-github-at-your-educational-institution/about-github-campus-program

## Install and Authenticate GitHub CLI

On macOS with Homebrew:

``` bash
brew install gh
```

Authenticate:

``` bash
gh auth login
```

Verify authentication:

``` bash
gh auth status
```

For a typical setup, authenticate to GitHub.com through the browser and
use SSH for Git operations.

### Required Organization Permissions

Some `mgc` operations, particularly organization custom-property
management, require the `admin:org` OAuth scope.

Add it to your existing GitHub CLI authentication:

``` bash
gh auth refresh -h github.com -s admin:org
```

Then verify:

``` bash
gh auth status
```

`mgc` does not maintain a separate GitHub token. Authentication and
credential storage are handled by `gh`.

## Install

### Download a release (no Go needed)

Prebuilt binaries for macOS, Linux, and Windows (amd64 and arm64) are on
the [Releases page](https://github.com/ArchitectingSoftware/my-gh-classroom/releases).
Each release has one archive per platform plus a checksums file:

``` text
mgc_1.0.0_darwin_arm64.tar.gz     macOS, Apple silicon
mgc_1.0.0_darwin_amd64.tar.gz     macOS, Intel
mgc_1.0.0_linux_amd64.tar.gz      Linux x86-64
mgc_1.0.0_linux_arm64.tar.gz      Linux ARM64
mgc_1.0.0_windows_amd64.zip       Windows x86-64
mgc_1.0.0_windows_arm64.zip       Windows ARM64
mgc_1.0.0_checksums.txt           SHA-256 of every archive
```

With `gh` (which `mgc` needs anyway), download, verify, and install on
macOS or Linux:

``` bash
gh release download --repo ArchitectingSoftware/my-gh-classroom \
  --pattern '*darwin_arm64.tar.gz' --pattern '*checksums.txt'
shasum -a 256 -c --ignore-missing mgc_*_checksums.txt   # must print OK
tar xzf mgc_*_darwin_arm64.tar.gz
sudo mv mgc_*_darwin_arm64/mgc /usr/local/bin/
mgc version
```

Change the pattern for your platform (`uname -sm` tells you which). On
Windows, unzip the archive and put `mgc.exe` somewhere on your `PATH`;
verify it with `Get-FileHash mgc_*_windows_amd64.zip` against the
checksums file.

**macOS:** the binaries are not signed by Apple, so a binary downloaded
with a browser is blocked the first time you run it ("cannot be opened
because the developer cannot be verified"). Clear the quarantine flag
once:

``` bash
xattr -d com.apple.quarantine /usr/local/bin/mgc
```

Files downloaded with `gh` or `curl` are not quarantined and run
directly.

### With Go

``` bash
go install github.com/ArchitectingSoftware/my-gh-classroom@latest
```

This installs to `$(go env GOPATH)/bin` as `my-gh-classroom`; rename it
to `mgc` if you like. Or clone the repository and see [Build](#build).

### First run

Create your config in `~/.mgc` and fill in the placeholders (see
[Creating a config](#creating-a-config)):

``` bash
mgc --apply init cs472 --home
```

## Build

Download dependencies:

``` bash
go mod tidy
```

Build with the Makefile:

``` bash
make
```

or:

``` bash
make build
```

Install `mgc` into `$(go env GOPATH)/bin`:

``` bash
make install
```

The resulting executable is:

``` text
mgc
```

You can also build directly:

``` bash
go build -o mgc .
```

Run the tests:

``` bash
make test
```

The tests use a fake GitHub CLI, so they need no network access or
`gh` authentication.

### Versions

``` bash
./mgc version       # version, commit, and Go/platform details
./mgc --version     # just the version
```

Versions come from git tags. `make build` and `make install` stamp the
binary with `git describe`, so a build of a tagged commit reports e.g.
`v1.0.0`, and later commits report e.g. `v1.0.0-3-gabc1234` (with
`-dirty` for uncommitted changes). `make version` prints the version a
build would get.

`mgc version` also shows the exact commit the binary was built from,
marked `(modified)` for uncommitted changes, which is the first thing to
ask for in a bug report. It works without a `config.json`.

### Cutting a release

Releases are built with the Makefile and published with `gh`. The
targets:

| Target         | What it does |
|----------------|--------------|
| `make dist`    | Cross-compiles all platforms into `dist/` as archives plus a checksums file, for **any** version. Use it to try a build; nothing is published. |
| `make release` | Same as `dist`, but only after `check-release`, `vet`, and `test` pass. |
| `make publish` | `make release`, then creates the GitHub release for the tag and uploads `dist/`. |

`make release` refuses to build unless:

- HEAD has a tag, and `VERSION` (defaults to `git describe`) is one of
  HEAD's tags;
- the tag is semver with a leading `v`: `v1.2.3`, or a prerelease such
  as `v1.2.3-rc.1`;
- the working tree is clean, including untracked files that are not
  gitignored (otherwise the binary reports itself as modified);
- `go.mod`/`go.sum` are tidy.

Release binaries are static (`CGO_ENABLED=0`), built with `-trimpath`
so no local paths are embedded, and stripped (`-s -w`). Each archive
contains `mgc` (`mgc.exe` on Windows), this README, and the LICENSE.

To publish:

``` bash
make test                                 # make sure main is good
git tag -a v1.0.0 -m "v1.0.0"
git push origin v1.0.0                    # the tag must be on GitHub first
make publish
```

`make publish` stops if a release for the tag already exists. Release
notes are generated from the commits and pull requests since the
previous release; edit them on GitHub afterward if you like. Tags with
a `-` (e.g. `v1.1.0-rc.1`) are published as prereleases, so they do not
become "Latest".

To fix a bad release, delete it (`gh release delete v1.0.0`), fix the
problem, and release a new patch version rather than moving the tag;
anyone who downloaded the old one would otherwise have a checksum that
no longer matches.

## Configuration

`mgc` supports multiple classrooms in one `config.json`.

### Creating a config

`mgc init` writes a starter config with one placeholder classroom:

``` bash
./mgc init cs472                   # preview the file
./mgc --apply init cs472           # write ./config.json
./mgc --apply init cs472 --home    # write ~/.mgc/config.json instead
```

The alias (`cs472` here) names the classroom and makes it the default;
without one it is `cs101`. Replace the `YOUR_*` placeholders, then run
`./mgc classroom verify` and `./mgc doctor`. Add more classrooms later
with [`classroom create`](#create-a-classroom).

`mgc init` never overwrites. If the file already exists it stops and
tells you to rename or delete it first. It also warns when another
config file with higher precedence (below) would be used instead of
the new one.

The repository ships [`config.example.json`](config.example.json),
which is exactly what `mgc init` generates. `config.json` itself is
gitignored so a personal config is never committed.

### Config file location

`mgc` uses the first of these that applies:

1. `--config PATH`
2. the `MGC_CONFIG` environment variable
3. `./config.json`, if it exists in the current directory
4. `~/.mgc/config.json`, if it exists
5. the OS config directory, if it exists:
   `~/Library/Application Support/mgc/config.json` on macOS,
   `~/.config/mgc/config.json` on Linux

`~/.mgc/config.json` is the recommended home for your config: an
installed `mgc` then works from any directory. A `./config.json` in the
current directory overrides it, which is handy for testing. `./mgc
doctor` prints the config file it is using.

A **classroom alias** is a short local name used by the CLI. It does not
have to match the GitHub organization name.

For example, the alias:

``` text
cs472
```

can refer to:

``` text
CS472-Net-WI26
```

A configuration can therefore look like:

``` json
{
  "default_classroom": "cs472",
  "classrooms": {
    "cs472": {
      "organization": "CS472-Net-WI26",
      "grader_team": "graders",
      "grader_permission": "push",
      "student_permission": "push",
      "course_info_repo": "course-info",
      "course_info_url": "https://github.com/CS472-Net-WI26/course-info",
      "course_name": "CS472",
      "repo_prefix": "",
      "repo_name_case": "lower",
      "instructors": ["Dr. Brian Mitchell"]
    },
    "cs281": {
      "organization": "CS281-Arch-FA26",
      "grader_team": "graders",
      "grader_permission": "push",
      "student_permission": "push",
      "course_info_repo": "course-info",
      "course_info_url": "https://github.com/CS281-Arch-FA26/course-info",
      "course_name": "CS281",
      "repo_prefix": "cs281",
      "repo_name_case": "lower",
      "instructors": ["Prof. Mitchell", "Prof. Jones"]
    }
  }
}
```

### Classroom Settings

| Setting              | Purpose                                                        |
|----------------------|----------------------------------------------------------------|
| `organization`       | GitHub organization containing this classroom’s repositories.  |
| `grader_team`        | GitHub team containing instructors and TAs.                    |
| `grader_permission`  | Permission granted to the grader team on student repositories. |
| `student_permission` | Direct permission granted to each student.                     |
| `course_info_repo`   | Shared course-information repository.                          |
| `course_info_url`    | URL inserted into each student’s initial README.               |
| `course_name`        | Optional course label for repo descriptions and READMEs. Defaults to the upper-cased alias. |
| `repo_prefix`        | Optional prefix for student repository names: `cs281` gives `cs281-jsmith42` (a `-` is added unless the prefix already ends in `-`, `_`, or `.`). Defaults to none. |
| `repo_name_case`     | `lower` (default) or `preserve`. Controls the case of generated student repository names; see [Repository name case](#repository-name-case). |
| `instructors`        | Optional list of names that sign `--message` notes, e.g. `["Dr. Brian Mitchell"]`. Defaults to “The <course_name> teaching team”. |

`default_classroom` contains the classroom alias used when
`--classroom`/`-c` is not supplied.

## Classroom Management

### List classrooms

``` bash
./mgc classroom list
```

The default classroom is identified in the output.

### Create a classroom

Preview:

``` bash
./mgc classroom create cs281
```

Apply:

``` bash
./mgc --apply classroom create cs281
```

Creating a classroom changes only the local configuration. It does
**not** create a GitHub organization.

A new classroom is created with obvious placeholder values:

``` json
{
  "organization": "YOUR_GITHUB_ORGANIZATION",
  "grader_team": "graders",
  "grader_permission": "push",
  "student_permission": "push",
  "course_info_repo": "YOUR_COURSE_INFO_REPO",
  "course_info_url": "https://github.com/YOUR_GITHUB_ORGANIZATION/YOUR_COURSE_INFO_REPO",
  "course_name": "CS281",
  "repo_prefix": "",
  "repo_name_case": "lower",
  "instructors": []
}
```

Edit `config.json` and replace the placeholders with the real
organization and repository information.

### Verify a classroom

After editing the configuration:

``` bash
./mgc classroom verify cs281
```

Verification checks the classroom configuration and corresponding GitHub
resources, including organization access, repository naming/URLs, the
course-info repository, and grader team.

Placeholder values such as `YOUR_GITHUB_ORGANIZATION` and
`YOUR_COURSE_INFO_REPO` should be replaced before verification.

### Set the default classroom

Show the current default:

``` bash
./mgc classroom default
```

Preview changing it:

``` bash
./mgc classroom default cs281
```

Apply:

``` bash
./mgc --apply classroom default cs281
```

The default is persisted in `config.json`.

If no default classroom is configured, `mgc` warns you and
classroom-scoped commands require an explicit classroom selection.

### Delete a classroom configuration

Preview:

``` bash
./mgc classroom delete cs281
```

Apply:

``` bash
./mgc --apply classroom delete cs281
```

**This removes only the classroom entry from `config.json`. It does not
delete the GitHub organization, repositories, teams, or student work.**

## Selecting the Active Classroom

Use the default classroom:

``` bash
./mgc student list
```

Or explicitly select one:

``` bash
./mgc --classroom cs472 student list
```

The convenience form is:

``` bash
./mgc -c cs472 student list
```

`-c` is the short form of `--classroom`.

Classroom-scoped commands print the active classroom and GitHub
organization so it is clear which course is being administered.

This is especially important for mutating operations.

## Doctor

Run:

``` bash
./mgc doctor
```

`doctor` checks the local environment, GitHub CLI authentication,
classroom selection, and the configured GitHub resources.

If a classroom is explicitly selected:

``` bash
./mgc -c cs281 doctor
```

the selected classroom is checked instead of the default.

## Safety Model

Mutating operations are dry-run by default.

For example:

``` bash
./mgc team create graders
```

previews the operation.

To perform it:

``` bash
./mgc --apply team create graders
```

The long form is also supported:

``` bash
./mgc --apply team create graders
```

This applies to both GitHub mutations and local configuration mutations.

## Grader Team Management

Create the configured grader team:

``` bash
./mgc team create graders
./mgc --apply team create graders
```

Add an instructor or TA:

``` bash
./mgc team add graders GITHUB_ID
./mgc --apply team add graders GITHUB_ID
```

If the person is not already in the organization, GitHub sends them an
organization invitation, and they are not on the team until they accept.
`team add` reports this as `INVITED` and prints the page where they
accept:

``` text
INVITED   ta-bo to 'graders'; they must accept the organization invitation
          accept at: https://github.com/orgs/CS472-Net-WI26/invitation (signed in as ta-bo)
```

Running `team add` again for someone whose invitation is still pending
is skipped, and the accept URL is shown again. If their invitation has
expired, `team add` sends a new one.

List graders, including people who have been invited but have not yet
accepted:

``` bash
./mgc team list graders
```

``` text
Members of CS472-Net-WI26/graders (1 active, 1 invited):
  ta-al
  ta-bo                    invited, 6d left
```

Inspect the team:

``` bash
./mgc team info graders
```

Remove a grader:

``` bash
./mgc team remove graders GITHUB_ID
./mgc --apply team remove graders GITHUB_ID
```

For someone who has not yet accepted, `team remove` cancels their
pending invitation instead. This cancels their organization invitation
as a whole.

### Lost TA and grader invitations

GitHub does not re-send the invitation email. When a TA or grader loses
it, send them the organization invitation page:

``` text
https://github.com/orgs/<organization>/invitation
```

They must be signed in to the invited GitHub account. To see everyone
on the grader team who has not accepted yet:

``` bash
./mgc team invites
```

``` text
Pending invitations to CS472-Net-WI26/graders (2, 1 expired or failed):

  GITHUB ID / EMAIL              STATUS             ACCEPT AT
  ta-bo                          pending, 6d left   https://github.com/orgs/CS472-Net-WI26/invitation
  ta-cy                          EXPIRED            (invite again)
```

Or one person, by GitHub username or invited email:

``` bash
./mgc team invites ta-bo
```

Add `--message` for a ready-to-paste note to each person (see
[Ready-to-Paste Messages](#ready-to-paste-messages)).

`--team NAME` reports on a team other than the classroom's
`grader_team`. Like student invitations, these expire after 7 days;
expired or failed invitations get no URL. Re-invite with `team add`,
which cancels the old invitation and sends a new one (`REINVITED`):

``` bash
./mgc --apply team add ta-cy
```

The grader team must exist before members can be added. `mgc`
deliberately avoids silently creating prerequisites as side effects.

An organization owner may also be a member of the grader team. Although
an owner already has administrative repository access, team membership
makes the instructional role explicit.

All of these commands operate on the active classroom, so another
classroom can be targeted with:

``` bash
./mgc -c cs281 team list graders
```

## Repository Custom Properties

`mgc` uses GitHub custom repository properties to associate a repository
with the student without requiring the student’s real name in the
repository name.

Student repositories use:

``` text
repo_type      = student
student_name   = Jane Smith
github_id      = jsmith42
```

Set up the property schema for the active classroom:

``` bash
./mgc properties setup
./mgc --apply properties setup
```

List the schema:

``` bash
./mgc properties list
```

Custom-property administration requires appropriate organization
permissions and typically the `admin:org` scope described earlier.

## Course Information Repository

Each classroom should have a shared `course-info` repository. Making it
public is convenient because students can read its instructions even
before accepting their private repository invitation.

Use it for stable information such as:

- repository organization;
- Git workflow;
- submission procedures; and
- instructions for getting help.

Assignments, grades, solutions, and other restricted content can remain
in the LMS.

Each newly provisioned student repository receives a small README,
titled with the classroom’s `course_name`, linking to its configured
`course_info_url`.

## Creating a Student Repository

Before provisioning the first student in a classroom, make sure the
grader team exists and the custom-property schema is set up (see
[Grader Team Management](#grader-team-management) and
[Repository Custom Properties](#repository-custom-properties)).
`./mgc doctor` and `./mgc properties list` will show both.

Preview:

``` bash
./mgc student create \
  --name "Jane Smith" \
  --github jsmith42
```

Apply:

``` bash
./mgc --apply student create \
  --name "Jane Smith" \
  --github jsmith42
```

By default the repository is named after the student’s GitHub ID,
preceded by the classroom’s `repo_prefix` if one is configured:

``` text
jsmith42            (no repo_prefix)
cs281-jsmith42      (repo_prefix "cs281" or "cs281-")
cs281_jsmith42      (repo_prefix "cs281_")
```

By default repository names are lowercase (`cs472-sudovoid1` for
GitHub user `SudoVoid1`); the student's exact GitHub ID is always kept
in the `github_id` property and used for their invitation. To keep the
case instead, see [Repository name case](#repository-name-case).
`--repo NAME` overrides the name for one student and is used exactly as
typed.

### Repository name case

Generated repository names (`<repo_prefix><GitHub ID>`) are lowercased
unless you ask otherwise. There are two ways to keep the case:

- **Per classroom:** set `"repo_name_case": "preserve"` in
  `config.json`. Every `student create` and `classroom import` for that
  classroom then keeps the case.
- **Per run:** add `--keep-case` to `student create` or
  `classroom import`. This overrides `"lower"` for that one command.

``` bash
./mgc --apply student create --name "Jane Smith" --github JSmith42 --keep-case
./mgc --apply classroom import roster.csv --keep-case
```

| `repo_prefix` | GitHub login | `lower` (default)  | `preserve` / `--keep-case` |
|---------------|--------------|--------------------|----------------------------|
| `CS281`       | `JSmith42`   | `cs281-jsmith42`   | `CS281-JSmith42`           |
| (none)        | `SudoVoid1`  | `sudovoid1`        | `SudoVoid1`                |

When case is kept, the GitHub ID part uses the login exactly as GitHub
reports it, not as the student typed it in the roster, so `jsmith42`
in the CSV still produces `CS281-JSmith42` if the account is
`JSmith42`.

GitHub treats repository names case-insensitively: `CS281-JSmith42` and
`cs281-jsmith42` are the same repository and cannot both exist. The
setting only changes how names look. Every lookup in `mgc` (import's
existing-repository check, `student info`, `student find`, `--repair`)
matches names case-insensitively, so mixing modes, switching the
setting mid-term, or renaming a repository's case on GitHub does not
cause duplicates or missed students. Changing the setting does not
rename existing repositories.

Prefer the config setting over the flag if you want preserved case
consistently; the flag is for the occasional one-off. `./mgc doctor`
and the import report header show which mode is active, e.g.
`Repo names: cs281-<github-id> (lowercase)`.

For a new repository, `mgc`:

1.  verifies the classroom and grader-team prerequisites;
2.  verifies that the GitHub account exists;
3.  checks whether the repository already exists;
4.  creates a private repository;
5.  initializes `README.md` with the `course-info` link;
6.  grants the grader team access;
7.  grants the student access (which sends the invitation); and
8.  assigns searchable student metadata.

If the repository already exists, `mgc` checks it and repairs whatever
is missing:

- the student has access. A pending invitation counts; an **expired**
  invitation is cancelled and a new one is sent (the student gets a new
  email and a working link);
- the grader team is connected;
- the custom properties are set (only empty ones are filled); and
- the course README is present, **only** if the repository is empty or
  its sole commit is GitHub’s auto-generated “Initial commit” with the
  placeholder `# <repo>` README.

Student work is never modified: once a repository has any other commit,
its contents are left alone. If an earlier run failed partway through,
re-running the same command finishes the setup.

`mgc` refuses to reuse an existing repository whose `github_id` property
names a different student, or whose `repo_type` is not `student`.

To provision into a non-default classroom:

``` bash
./mgc -c cs281 --apply student create \
  --name "Jane Smith" \
  --github jsmith42
```

## Student Inspection

List student repositories:

``` bash
./mgc student list
```

Only repositories with `repo_type=student` are listed; the number of
other repositories in the organization is noted at the end.

Show details for a student, by GitHub ID or repository name:

``` bash
./mgc student info jsmith42
```

The detailed view includes repository information, student permission,
grader-team access, and pending invitations when available.

## Finding a Student

Search by part of a student’s name:

``` bash
./mgc student find "Smith"
```

Search by full name:

``` bash
./mgc student find "Jane Smith"
```

Search by GitHub ID:

``` bash
./mgc student find "jsmith42"
```

Search is case-insensitive, supports partial matches, and covers
repositories with `repo_type=student`.

You can search another classroom explicitly:

``` bash
./mgc -c cs281 student find "Smith"
```

## Importing Students from a CSV Roster

`classroom import` provisions students from a CSV file:

``` bash
./mgc classroom import roster.csv                   # dry run, whole file
./mgc classroom import roster.csv -n 2              # dry run, first 2 students
./mgc --apply classroom import roster.csv           # create repositories
./mgc --apply classroom import roster.csv --repair  # create, and fix incomplete repos
./mgc -c cs281 --apply classroom import roster.csv  # a non-default classroom
```

### Roster format

By default the header row must include these two columns, spelled
exactly:

| Column      | Contents                                  |
|-------------|-------------------------------------------|
| `Name`      | Student name, e.g. `Jane Smith`           |
| `GitHub-ID` | Student GitHub username, e.g. `jsmith42`  |

If your file uses different headers, name them instead:

``` bash
./mgc classroom import export.csv --name-column Student --github-column "GitHub Username"
```

Any other columns are ignored and column order does not matter, so an
LMS export can be used unchanged (see
[Suggested Canvas Workflow](#suggested-canvas-workflow)). A minimal
roster is just:

``` text
Name,GitHub-ID
Jane Smith,jsmith42
Bob Baker,bbaker
```

The file is checked for both columns before anything is sent to
GitHub. `./mgc classroom import --help` lists the required columns.

Each new repository is named `<repo_prefix><GitHub ID>` using the
classroom’s `repo_prefix` (see [Classroom Settings](#classroom-settings)),
or just the GitHub ID when no prefix is set. Names are lowercase unless
the classroom sets `"repo_name_case": "preserve"` or you pass
`--keep-case` (see [Repository name case](#repository-name-case)). The
report header shows the resolved pattern, e.g.
`Repo names: cs472-<github-id> (lowercase)`.
Running `classroom import` with no file prints a warning and usage.

`-n`/`--number N` processes only the first N students, which is handy
for trying things out. Without it the whole file is processed.

Import is an **upsert**. A student who already has a repository is
skipped and their repository is not changed in any way. Existing
repositories are found by the `github_id` custom property, then by
`<repo_prefix><GitHub ID>`, then by the bare GitHub ID (a repository
created before a prefix was configured). The same export can be
imported again and again as more students fill in the survey. The same
GitHub ID appearing twice in the file is only processed once.

With `--repair`, existing repositories are checked instead of skipped,
and anything missing is fixed exactly as described in
[Creating a Student Repository](#creating-a-student-repository). Like
everything else, `--repair` is a dry run without `--apply`, so
`./mgc classroom import roster.csv --repair` shows what would be fixed.

Before processing students, import checks that the grader team exists
and the custom-property schema is set up. A missing team stops the run.
Missing properties stop an `--apply` run and produce a warning in a dry
run.

Common entry mistakes are cleaned up: a leading `@` or a full
`https://github.com/...` URL becomes the bare username, and the report
notes what the student originally typed. Blank or malformed IDs, and
accounts that do not exist on GitHub, are reported as errors without
stopping the run.

A report is printed as each student is processed:

``` text
[ 1/44] Jane Smith                   jsmith42               CREATING     SUCCESS  https://github.com/CS472-Net-WI26/jsmith42
[ 2/44] Bob Baker                    bbaker                 SKIPPING     SUCCESS  repository exists: https://github.com/CS472-Net-WI26/bbaker
[ 3/44] Carol Chen                   cchen-typo             CHECKING     ERROR    GitHub user 'cchen-typo' does not exist
[ 4/44] Dave Diaz                    dave                   REPAIRING    SUCCESS  dave: set custom properties: github_id=dave, ...
```

The statuses are `CREATING` / `WOULD CREATE`, `REPAIRING` /
`WOULD REPAIR` (with `--repair`), `SKIPPING`, and `CHECKING` (the
student failed validation before any action). A summary follows, with
any errors repeated alongside their CSV row numbers.

The same report is written to `import-results.txt` in the current
directory, which is overwritten on each run. It contains student names,
so it is listed in `.gitignore`. The command exits non-zero if any
student had an error.

Add `--message` to get a note for each student whose GitHub username
was blank, invalid, or not found on GitHub, asking them to resubmit
(see [Ready-to-Paste Messages](#ready-to-paste-messages)). Problems only
you can fix, such as a repository name conflict, get no message.

If a repository is created but a later setup step fails, the error line
says so and the run continues with the next student. Re-run with
`--repair` to finish it:

``` bash
./mgc classroom import roster.csv --repair          # review what will be fixed
./mgc --apply classroom import roster.csv --repair
```

## Suggested Canvas Workflow

A simple workflow is:

1.  Create a short Canvas survey asking each student for their **GitHub
    username**.
2.  Explicitly state that this is their GitHub username, not their
    university ID or email address.
3.  Ask students to visit GitHub and verify that they can log into the
    account before submitting.
4.  Export the Canvas Student Analysis results as CSV. Check the header
    row: if the GitHub column is headed with the question text rather
    than `GitHub-ID`, rename it or pass `--github-column "<header>"`.
5.  Run `./mgc classroom import Canvas-Export.csv` (a dry run) and
    review the report, especially any `ERROR` lines.
6.  Optionally try a couple of students for real:
    `./mgc --apply classroom import Canvas-Export.csv -n 2`.
7.  Run `./mgc --apply classroom import Canvas-Export.csv`.
8.  Re-export and re-run later for students who were absent or had
    account problems. Students already provisioned are skipped. If the
    report shows any setup errors, add `--repair`.

## Student Invitations

When a student is added as a collaborator to a private repository,
GitHub emails them an invitation, and they must accept it before they
can use the repository. GitHub does **not** re-send that email, so when
a student loses or ignores it, send them the page where they can accept
directly:

``` text
https://github.com/<organization>/<repository>/invitations
```

They must be signed in to the GitHub account that was invited.

`mgc` can list these for you. For everyone in the classroom with a
pending invitation, sorted by student name:

``` bash
./mgc student invites
```

``` text
Pending repository invitations in CS472-Net-WI26 (3, 1 expired):

  STUDENT                      GITHUB ID              STATUS             ACCEPT AT
  Alice Anders                 alice1                 pending, 6d left   https://github.com/CS472-Net-WI26/alice1/invitations
  Bob Baker                    bbaker                 pending, 2d left   https://github.com/CS472-Net-WI26/bbaker/invitations
  Dave Diaz                    dave                   EXPIRED            (expired: invite again)
```

For one student, by GitHub ID or repository name:

``` bash
./mgc student invites jsmith42
```

This shows when the invitation was sent, when it expires, and the
accept URL, or tells you the student has already accepted. Students who
have accepted do not appear in the organization-wide list.
`./mgc student info` also shows the accept URL for any pending
invitation. Add `--message` to get a ready-to-paste note for each
student (see [Ready-to-Paste Messages](#ready-to-paste-messages)).

GitHub invitations **expire after 7 days**. Expired invitations are
marked `EXPIRED` and no URL is offered, because the link no longer
works. Re-invite them with the roster import's repair, which cancels
each expired invitation and sends a fresh one:

``` bash
./mgc classroom import roster.csv --repair            # review first
./mgc --apply classroom import roster.csv --repair
```

For one student, `./mgc student invites GITHUB_ID` prints the exact
`mgc --apply student create ...` command that re-invites them.

## Ready-to-Paste Messages

`--message` turns a report into short notes you can paste into an email
or a Canvas message. `mgc` never sends anything itself.

``` bash
./mgc student invites --message                # students who have not accepted yet
./mgc team invites --message                   # TAs and graders who have not accepted yet
./mgc classroom import roster.csv --message    # students whose GitHub username needs fixing
```

Each note is printed as a block with a `To:` line and a subject, and all
of them are also saved to `messages.txt` in the current directory
(overwritten on every run; it contains names, so keep it out of git):

``` text
========================================================================
To:      Jane Smith (GitHub: jsmith42)
Subject: CS472: accept your GitHub repository invitation

Hi Jane,

Your CS472 repository is ready. Accept the invitation here
(sign in to GitHub as jsmith42 first):

https://github.com/CS472-Net-WI26/jsmith42/invitations

The invitation expires on Tuesday, October 6.

Dr. Brian Mitchell
========================================================================
```

Notes are signed with the classroom's `instructors`: one name as is,
two as “A and B”, three or more as “A, B, and C”. With no instructors
configured they are signed “The <course_name> teaching team”.

Only invitations that can still be accepted get a note. Re-invite
expired ones first (see [Student Invitations](#student-invitations)),
then run `--message` again.

## Example: Two Concurrent Classes

Suppose the configuration contains:

``` text
cs472 -> CS472-Net-WI26
cs281 -> CS281-Arch-FA26
```

and `cs472` is the default.

These commands operate on CS472:

``` bash
./mgc student list
./mgc student find "Smith"
```

These explicitly operate on CS281:

``` bash
./mgc -c cs281 student list
./mgc -c cs281 student find "Smith"
```

Before provisioning a student, the output identifies the active
classroom and organization so the target is visible before `--apply` is
used.

## Design Principles

**Use GitHub as GitHub.** Students work with ordinary Git repositories,
commits, pushes, and URLs.

**Support multiple concurrent courses.** A classroom alias selects the
GitHub organization and settings for a specific course offering.

**One repository per student.** Individual assignments are directories
inside the student’s course repository.

**Centralize grader access.** Instructors and TAs belong to one GitHub
team per classroom rather than being added individually to every
repository.

**Never overwrite student work during provisioning.** The contents of
existing repositories are never modified; only missing access grants
and metadata are repaired.

**Prefer explicit operations.** Missing prerequisites produce errors
rather than silently creating unrelated resources.

**Dry-run first.** Changes require an explicit `--apply`/`-a`.

**Keep credentials out of the utility.** Authentication is delegated to
the official GitHub CLI.

## Project Layout

``` text
.
├── cmd/                 Cobra commands (one file per command group)
│   ├── classroom.go
│   ├── import.go        classroom import (CSV roster)
│   ├── doctor.go
│   ├── init.go          mgc init (starter config)
│   ├── properties.go
│   ├── root.go
│   ├── student.go
│   └── team.go
├── internal/
│   ├── roster/          CSV roster parsing
│   ├── config/          config loading, validation, and path resolution
│   ├── course/          classroom operations (provisioning, search, teams)
│   └── gh/              thin wrapper around the `gh` CLI
├── main.go
├── config.example.json  starter config, identical to `mgc init` output
├── go.mod
├── LICENSE
├── Makefile
└── README.md
```

Configuration persistence uses Go’s standard JSON support. Cobra
provides the command/subcommand structure, and `gh` provides
authenticated GitHub API access.

Each package has `_test.go` files alongside it. `internal/course` tests
run against a fake `gh` (`fake_gh_test.go`) that records every API call,
so tests can assert exactly which mutations a command performs,
including that dry runs perform none.

## Dependencies

- [Cobra](https://github.com/spf13/cobra) for CLI commands and flags.
- [GitHub CLI](https://cli.github.com/) for authentication and GitHub
  API access.

## Current Status

The core multi-classroom configuration, grader-team administration,
custom-property setup, one-off student provisioning, student
inspection/search, roster import, and CSV batch provisioning workflows
are implemented and covered by unit tests.

### Upgrading from earlier versions

Earlier versions did not send the request body when setting custom
properties, so student repositories created with them may have no
`repo_type`, `student_name`, or `github_id` values. Those repositories
will not appear in `student list` or `student find`. To fix them, run
the roster import with `--repair` (or `student create` for a single
student): it detects the missing properties and sets them without
touching repository contents. Review the dry run first.

`student create-batch` has been removed; use `classroom import`, with
`--name-column`/`--github-column` if your CSV headers differ.

## License

MIT. See [LICENSE](LICENSE).
