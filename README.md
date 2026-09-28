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
> by default**. Pass `--apply` or `-apply` explicitly to perform a
> mutation.

## Features

- Manage multiple classrooms from one installation.
- Give each classroom a short local alias such as `cs472` or `cs281`.
- Select a classroom globally with `--classroom` or `-cr`.
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
- Import students from a Canvas export (`classroom import`), as a
  repeatable upsert with an on-screen and saved report.
- Batch-provision students from generic CSV data.
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

## Configuration

`mgc` supports multiple classrooms in one `config.json`.

### Config file location

`mgc` uses the first of these that applies:

1. `--config PATH`
2. the `MGC_CONFIG` environment variable
3. `./config.json`, if it exists in the current directory
4. the per-user config directory: `~/Library/Application Support/mgc/config.json`
   on macOS, `~/.config/mgc/config.json` on Linux

Options 2 and 4 let an installed `mgc` run from any directory.

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
      "course_info_url": "https://github.com/CS472-Net-WI26/course-info"
    },
    "cs281": {
      "organization": "CS281-Arch-FA26",
      "grader_team": "graders",
      "grader_permission": "push",
      "student_permission": "push",
      "course_info_repo": "course-info",
      "course_info_url": "https://github.com/CS281-Arch-FA26/course-info",
      "course_name": "CS281"
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

`default_classroom` contains the classroom alias used when
`--classroom`/`-cr` is not supplied.

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
./mgc -apply classroom create cs281
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
  "course_info_url": "https://github.com/YOUR_GITHUB_ORGANIZATION/YOUR_COURSE_INFO_REPO"
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
./mgc -apply classroom default cs281
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
./mgc -apply classroom delete cs281
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
./mgc -cr cs472 student list
```

`-cr` is accepted as a convenience alias for `--classroom`.

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
./mgc -cr cs281 doctor
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
./mgc -apply team create graders
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
./mgc -apply team create graders
```

Add an instructor or TA:

``` bash
./mgc team add graders GITHUB_ID
./mgc -apply team add graders GITHUB_ID
```

List graders:

``` bash
./mgc team list graders
```

Inspect the team:

``` bash
./mgc team info graders
```

Remove a grader:

``` bash
./mgc team remove graders GITHUB_ID
./mgc -apply team remove graders GITHUB_ID
```

The grader team must exist before members can be added. `mgc`
deliberately avoids silently creating prerequisites as side effects.

An organization owner may also be a member of the grader team. Although
an owner already has administrative repository access, team membership
makes the instructional role explicit.

All of these commands operate on the active classroom, so another
classroom can be targeted with:

``` bash
./mgc -cr cs281 team list graders
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
./mgc -apply properties setup
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
./mgc -apply student create \
  --name "Jane Smith" \
  --github jsmith42
```

By default, the student’s GitHub ID becomes the repository name:

``` text
jsmith42
```

For a new repository, `mgc`:

1.  verifies the classroom and grader-team prerequisites;
2.  verifies that the GitHub account exists;
3.  checks whether the repository already exists;
4.  creates a private repository;
5.  initializes `README.md` with the `course-info` link;
6.  grants the grader team access;
7.  grants the student access (which sends the invitation); and
8.  assigns searchable student metadata.

If the repository already exists, its contents are never touched. `mgc`
instead checks that the student has access (or a pending invitation),
that the grader team is connected, and that the custom properties are
set, and repairs whatever is missing. If an earlier run failed partway
through, re-running the same command finishes the setup. The initial
README is only written when the repository is first created.

`mgc` refuses to reuse an existing repository whose `github_id` property
names a different student, or whose `repo_type` is not `student`.

To provision into a non-default classroom:

``` bash
./mgc -cr cs281 -apply student create \
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
./mgc -cr cs281 student find "Smith"
```

## Importing Students from Canvas

`classroom import` provisions students from a Canvas survey export:

``` bash
./mgc classroom import Canvas-Export.csv            # dry run, whole file
./mgc classroom import Canvas-Export.csv -n 2       # dry run, first 2 students
./mgc -apply classroom import Canvas-Export.csv     # create repositories
./mgc -cr cs281 -apply classroom import roster.csv  # a non-default classroom
```

The CSV must have a `Name` column (e.g. `Jane Smith`) and a `GitHub-ID`
column. Other columns are ignored. The file is checked for both columns
before anything is sent to GitHub. Running `classroom import` with no
file prints a warning and usage.

`-n`/`--number N` processes only the first N students, which is handy
for trying things out. Without it the whole file is processed.

Import is an **upsert**. A student who already has a repository, found
by the `github_id` custom property or by a repository named after their
GitHub ID, is skipped and their repository is not changed in any way.
The same export can be imported again and again as more students fill
in the survey. The same GitHub ID appearing twice in the file is only
processed once.

Before processing students, import checks that the grader team exists
and the custom-property schema is set up. A missing team stops the run.
Missing properties stop an `-apply` run and produce a warning in a dry
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
```

The statuses are `CREATING`, `WOULD CREATE` (dry run), `SKIPPING`, and
`CHECKING` (the student failed validation before any action). A summary
follows, with any errors repeated alongside their CSV row numbers.

The same report is written to `import-results.txt` in the current
directory, which is overwritten on each run. It contains student names,
so it is listed in `.gitignore`. The command exits non-zero if any
student had an error.

If a repository is created but a later setup step fails, the error line
includes the `mgc -apply student create ...` command that finishes the
setup. Because import never modifies existing repositories, use that
command (not another import) to repair it.

## Batch Student Provisioning

Generic CSV provisioning, with configurable column names, is also
available:

``` bash
./mgc student create-batch students.csv \
  --github-column "GitHub Username" \
  --name-column "Student"
```

Review the dry run, then apply:

``` bash
./mgc -apply student create-batch students.csv \
  --github-column "GitHub Username" \
  --name-column "Student"
```

Existing repositories are skipped (and repaired if needed), so the same
CSV can be processed repeatedly as students complete setup. A UTF-8 BOM
on the header row and short rows are tolerated.

Unlike `classroom import`, `create-batch` repairs missing access and
metadata on existing repositories.

## Suggested Canvas Workflow

A simple workflow is:

1.  Create a short Canvas survey asking each student for their **GitHub
    username**.
2.  Explicitly state that this is their GitHub username, not their
    university ID or email address.
3.  Ask students to visit GitHub and verify that they can log into the
    account before submitting.
4.  Export the Canvas Student Analysis results as CSV.
5.  Run `./mgc classroom import Canvas-Export.csv` (a dry run) and
    review the report, especially any `ERROR` lines.
6.  Optionally try a couple of students for real:
    `./mgc -apply classroom import Canvas-Export.csv -n 2`.
7.  Run `./mgc -apply classroom import Canvas-Export.csv`.
8.  Re-export and re-run later for students who were absent or had
    account problems. Students already provisioned are skipped.

## Student Invitations

When a student is added as a collaborator to a private repository,
GitHub handles its normal repository invitation/notification flow.

Students must accept the invitation before using the repository.

If a student loses the invitation email, have them sign into the GitHub
account they supplied and check their GitHub notifications.

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
./mgc -cr cs281 student list
./mgc -cr cs281 student find "Smith"
```

Before provisioning a student, the output identifies the active
classroom and organization so the target is visible before `-apply` is
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

**Dry-run first.** Changes require an explicit `-apply`/`--apply`.

**Keep credentials out of the utility.** Authentication is delegated to
the official GitHub CLI.

## Project Layout

``` text
.
├── cmd/                 Cobra commands (one file per command group)
│   ├── classroom.go
│   ├── import.go        classroom import (Canvas)
│   ├── doctor.go
│   ├── properties.go
│   ├── root.go
│   ├── student.go
│   └── team.go
├── internal/
│   ├── canvas/          Canvas export parsing
│   ├── config/          config loading, validation, and path resolution
│   ├── course/          classroom operations (provisioning, search, teams)
│   └── gh/              thin wrapper around the `gh` CLI
├── main.go
├── config.json
├── go.mod
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
inspection/search, Canvas import, and CSV batch provisioning workflows
are implemented and covered by unit tests.

### Upgrading from earlier versions

Earlier versions did not send the request body when setting custom
properties, so student repositories created with them may have no
`repo_type`, `student_name`, or `github_id` values. Those repositories
will not appear in `student list` or `student find`. To fix them,
re-run the original `student create` or `student create-batch` command:
it detects the missing properties and sets them without touching
repository contents. Review the dry run first.
