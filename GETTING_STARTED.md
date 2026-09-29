# Getting Started with mgc

This guide walks you through a first term with `mgc`, from installing it
to having a private repository for every student. It covers what to do
and in what order; the [README](README.md) has the full reference for
every command.

> **Everything is a dry run until you add `--apply`.** Every command that
> changes something on GitHub or on disk first shows what it *would* do.
> Run it once to review, then run it again with `--apply` (or `-a`).
> This guide shows both where it matters.

**What you end up with:**

- a GitHub organization for your course;
- a public `course-info` repository with course-wide instructions;
- a `graders` team with your TAs, with access to every student repository;
- one private repository per student (e.g. `cs472-jsmith42`), which the
  student and graders can access and nobody else can see.

## Contents

1. [Get mgc](#1-get-mgc)
2. [Set up the GitHub CLI and your config](#2-set-up-the-github-cli-and-your-config)
3. [Set up a GitHub organization](#3-set-up-a-github-organization)
4. [Create the course-info repository](#4-create-the-course-info-repository)
5. [Add graders](#5-add-graders)
6. [Create student repositories from a roster](#6-create-student-repositories-from-a-roster)
7. [During the term](#7-during-the-term)
8. [Troubleshooting](#8-troubleshooting)
9. [Checklist for each term](#9-checklist-for-each-term)

## 1. Get mgc

### Download a release (recommended)

Download the archive for your platform from the
[Releases page](https://github.com/ArchitectingSoftware/my-gh-classroom/releases),
e.g. `mgc_1.0.0_darwin_arm64.tar.gz` for an Apple silicon Mac. Once you
have `gh` installed (step 2), this downloads, verifies, and installs it:

``` bash
gh release download --repo ArchitectingSoftware/my-gh-classroom \
  --pattern '*darwin_arm64.tar.gz' --pattern '*checksums.txt'
shasum -a 256 -c --ignore-missing mgc_*_checksums.txt   # must print OK
tar xzf mgc_*_darwin_arm64.tar.gz
sudo mv mgc_*_darwin_arm64/mgc /usr/local/bin/
mgc version
```

Use `darwin_amd64` for an Intel Mac, `linux_amd64` or `linux_arm64` for
Linux. On Windows, unzip `mgc_*_windows_amd64.zip` and put `mgc.exe` on
your `PATH`.

**macOS:** if you downloaded with a browser and macOS refuses to run
`mgc` ("the developer cannot be verified"), clear the quarantine flag
once:

``` bash
xattr -d com.apple.quarantine /usr/local/bin/mgc
```

### Or build from source

You need Go 1.23 or newer.

``` bash
git clone https://github.com/ArchitectingSoftware/my-gh-classroom.git
cd my-gh-classroom
make build          # ./mgc
make install        # or install to $(go env GOPATH)/bin
```

The rest of this guide writes `mgc`; use `./mgc` if you run it from the
source directory.

## 2. Set up the GitHub CLI and your config

### The GitHub CLI

`mgc` does all its GitHub work through the official GitHub CLI, `gh`,
and never stores a token of its own. Install and log in:

``` bash
brew install gh                     # macOS; see https://cli.github.com for others
gh auth login                       # GitHub.com, log in with a browser
gh auth refresh -h github.com -s admin:org
gh auth status
```

The `admin:org` scope is required: `mgc` uses it to set up the
organization's custom properties (step 3).

### Your config file

`mgc` keeps your course settings in a `config.json`. Create one in
`~/.mgc`, where `mgc` finds it from any directory:

``` bash
mgc init cs472 --home               # preview
mgc --apply init cs472 --home       # write ~/.mgc/config.json
```

`cs472` is the **classroom alias**: a short name you use on the command
line. It does not need to match anything on GitHub. The file looks like
this:

``` json
{
  "default_classroom": "cs472",
  "classrooms": {
    "cs472": {
      "organization": "YOUR_GITHUB_ORGANIZATION",
      "grader_team": "graders",
      "grader_permission": "push",
      "student_permission": "push",
      "course_info_repo": "YOUR_COURSE_INFO_REPO",
      "course_info_url": "https://github.com/YOUR_GITHUB_ORGANIZATION/YOUR_COURSE_INFO_REPO",
      "course_name": "CS472",
      "repo_prefix": "",
      "repo_name_case": "lower",
      "instructors": []
    }
  }
}
```

You fill in the `YOUR_*` values in steps 3 and 4. The other settings you
are most likely to change now:

| Setting          | Suggestion |
|------------------|------------|
| `course_name`    | How the course is named in student READMEs, e.g. `CS472`. |
| `repo_prefix`    | e.g. `cs472` names repositories `cs472-<github-id>`. Recommended: it makes student repos easy to spot and sort. |
| `instructors`    | Your name(s), e.g. `["Dr. Jane Doe"]`, used to sign messages `mgc` writes for students. |

Leave the rest at their defaults; the README's
[Classroom Settings](README.md#classroom-settings) explains each one.

Teaching more than one course? Add another classroom with
`mgc --apply classroom create cs281`, and pick one per command with
`-c cs281`. The `default_classroom` is used when you don't.

## 3. Set up a GitHub organization

Each course gets its own GitHub organization, which holds the student
repositories, the grader team, and course-info.

**One organization per course per term** (e.g. `CS472-Net-WI26`) is the
simplest model: each term starts clean, and old terms stay intact as a
record. You can reuse one organization across terms, but then old and
new student repositories mix.

1. **Apply for GitHub Education first** (optional, but worth it).
   Verified teachers get benefits useful for courses with private
   repositories:
   https://docs.github.com/en/education/about-github-education/github-education-for-teachers
2. **Create the organization** at https://github.com/account/organizations/new.
   Pick a name that includes the course and term.
3. **Review member privileges** under the organization's
   *Settings → Member privileges*. We suggest setting **Base
   permissions** to **No permission**, so graders see student
   repositories through the `graders` team rather than through org
   membership, and leaving **Allow forking of private repositories**
   off.
4. **Put the organization name in your config** (`organization`).
5. **Set up the custom properties** `mgc` uses to link each repository to
   its student:

   ``` bash
   mgc properties setup
   mgc --apply properties setup
   ```

   These properties (`repo_type`, `student_name`, `github_id`) are how
   `mgc` finds a student's repository without putting their real name in
   the repository name. Skip this and `student list` and `student find`
   won't see anyone.

## 4. Create the course-info repository

Every student repository `mgc` creates starts with a short README that
links to one shared repository, `course-info` by default. It is the
single place for instructions that apply to the whole course, so you
update them once instead of in 50 repositories.

Make it **public**: students can then read it before they accept their
repository invitation, and it needs no access management.

Good things to put in it:

- how to clone and set up their repository;
- how you want the repository organized, e.g. one folder per assignment
  (`hw1/`, `hw2/`, ...);
- the Git workflow and how to submit (e.g. "push by the deadline; the
  last commit before the deadline is graded");
- how to get help.

Keep grades, solutions, and anything private in your LMS.

Create it and point your config at it:

``` bash
gh repo create CS472-Net-WI26/course-info --public --add-readme
```

``` json
"course_info_repo": "course-info",
"course_info_url": "https://github.com/CS472-Net-WI26/course-info",
```

Now check that everything so far is in place:

``` bash
mgc classroom verify      # organization, course-info, and grader team on GitHub
mgc doctor                # gh login, active classroom, and which config file is in use
```

`verify` stops with an error at the grader team until you create it in
step 5; everything before that line should say `OK`.

## 5. Add graders

Instructors and TAs go in a **grader team**. `mgc` gives the team access
to every student repository it creates, so adding a TA once gives them
access to all current and future student repositories.

``` bash
mgc team create graders                      # preview
mgc --apply team create graders

mgc team add graders ta-github-id            # preview
mgc --apply team add graders ta-github-id
```

If the TA isn't in the organization yet, GitHub emails them an
organization invitation, and `mgc` reports `INVITED` along with the page
where they accept. **They have no access until they accept.**

``` bash
mgc team list graders                        # who's on the team, including pending invites
mgc team invites --message                   # anyone who hasn't accepted, plus a note to send them
mgc --apply team remove graders ta-github-id # when a TA leaves
```

`grader_permission` (default `push`) is what the team gets on each
student repository: `push` lets graders commit feedback, `pull` makes
them read-only.

## 6. Create student repositories from a roster

### Collect GitHub usernames

Ask students for their GitHub username, e.g. with a short survey in your
LMS. From experience:

- say explicitly that you want their **GitHub username**, not their
  university ID or email;
- ask them to log in to GitHub first to check the account works.

You need a CSV with a name column and a GitHub username column. By
default `mgc` looks for headers named `Name` and `GitHub-ID`. If your
export uses other headers (Canvas uses the question text, for example),
pass them with `--name-column` and `--github-column`, or rename them in
the file. Other columns are ignored.

``` text
Name,GitHub-ID
Jane Smith,jsmith42
Bob Baker,bbaker
```

### Import

Review first. The dry run checks every username on GitHub and shows
exactly what would be created:

``` bash
mgc classroom import roster.csv
```

Look at any `ERROR` lines (blank, malformed, or nonexistent usernames).
Add `--message` to get a ready-to-paste note for each of those students
asking them to fix their username.

Then try two students for real, check the results on GitHub, and run the
rest:

``` bash
mgc --apply classroom import roster.csv -n 2
mgc --apply classroom import roster.csv
```

Import is an **upsert**: students who already have a repository are
skipped. Re-run it with an updated export as late students respond.
Each run's report is also saved to `import-results.txt`.

For each student, `mgc` creates a private repository, invites the
student, gives the grader team access, adds the course README, and
records the student's name and GitHub username as custom properties.

### Tell your students

GitHub emails each student an invitation to their repository. **They
must accept it within 7 days**, and GitHub does not re-send it. A short
announcement helps:

``` text
Your personal CS472 repository has been created in our GitHub
organization. It's named cs472-<your-github-id>, and you'll use it for
all assignments this term.

GitHub has emailed an invitation to the address on your GitHub account.
Please accept it within 7 days, since invitations expire after that.
If you can't find the email, sign in to GitHub and go to:
https://github.com/CS472-Net-WI26/cs472-<your-github-id>/invitations

Once you've accepted, your repo's README links to the course info page
with setup and submission instructions.
```

About a week later, find who hasn't accepted:

``` bash
mgc student invites --message
```

This lists everyone with a pending invitation, with their accept link,
and writes a note for each one.

## 7. During the term

| Task | Command |
|------|---------|
| Add a late student | `mgc --apply student create --name "Jane Smith" --github jsmith42` |
| Look up a student | `mgc student info jsmith42`, or `mgc student find smith` by name, username, or repo |
| List all student repositories | `mgc student list` |
| Chase unaccepted invitations | `mgc student invites --message` |
| Re-invite students whose invitation expired | `mgc --apply classroom import roster.csv --repair` |
| Fix a repository whose setup didn't finish | `mgc --apply classroom import roster.csv --repair` |
| Check your setup | `mgc doctor` |

`--repair` checks existing repositories and fixes anything missing:
student access, grader access, custom properties, and expired
invitations. It never touches student work. Like everything else, run it
without `--apply` first to see what it would do.

## 8. Troubleshooting

**`config file not found`**: `mgc` looks for `--config`, then
`$MGC_CONFIG`, then `./config.json`, then `~/.mgc/config.json`. Create
one with `mgc --apply init <alias> --home`. `mgc doctor` shows which file
is in use.

**`gh` errors about authentication or scopes**: run `gh auth status`.
If `admin:org` is missing, run
`gh auth refresh -h github.com -s admin:org`.

**A student says they have no access**: they haven't accepted the
invitation, or they accepted with a different GitHub account.
`mgc student info <username>` shows whether an invitation is pending and
its accept link.

**A student gave the wrong username**: create their repository with the
right one (`mgc --apply student create ...`). If a repository was
already created under the wrong account, delete it on GitHub first.

**`student list` or `student find` doesn't show a student**: the
repository is probably missing its custom properties, e.g. it was
created before `mgc properties setup` was run. Fix it with `--repair`.

**macOS won't run the downloaded binary**: see the `xattr` command in
[step 1](#download-a-release-recommended).

## 9. Checklist for each term

- [ ] Create the organization, e.g. `CS472-Net-SP27` (step 3)
- [ ] Add a classroom for it: `mgc --apply classroom create cs472-sp27`,
      or update the existing entry in `~/.mgc/config.json`
- [ ] `mgc --apply properties setup`
- [ ] Create and fill in `course-info` (step 4)
- [ ] `mgc classroom verify` and `mgc doctor`
- [ ] Create the grader team and add TAs (step 5)
- [ ] Survey students for GitHub usernames
- [ ] Import: dry run, `-n 2`, then everyone (step 6)
- [ ] Post the announcement
- [ ] After a week: `mgc student invites --message`
