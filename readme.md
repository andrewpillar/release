# release

release is a simple utility for preparing and tagging releases via git whilst
adhering to [Semantic Versioning][0]. The usage of release is as follows,

    release [-info] <major|minor|patch> [pre-release]

where `<major|minor|patch>` specifies how we want to bump the release, `-info`
denotes whether build information should be included, and `pre-release` is the
pre-release suffix to add.

[0]: https://semver.org/

When a new release is prepared, an editor will be opened up via `EDITOR` in
which release notes can be written. These would be a high-level overview of the
changes made in the release. The output of `git shortlog` is then appended to
these notes, which are then used for the final message for the tag. The editor
is then opened one final time wherein the final tag message can be amended. An
archive of the tag will also be created via `git archive` with the name of
`<tag>.tar.gz`.

## Usage

Let's assume we have a piece of software we want to issue an initial pre-release
for. With this release, we want to mark it as a pre-release, and include the
build information too. To do this with `release`, we would run the following,

    $ release -info patch alpha

after having written up the release notes we will see the version that was just
tagged,

    $ release -info patch alpha
    v0.0.1-alpha+1a2b3c4

we can then view the full release notes via `git show`,

    $ git show v0.0.1-alpha+1a2b3c4
    Pre-release build.

    Changelog:

    Andrew Pillar (1):
        Initial commit

this will also produce an archive of the newly created tag called
`v0.0.1-alpha+1a2b3c4.tar.gz`. The latest release can then be bumped a version.
Assume we have made some more changes, and the software is stable enough to be
considered out of alpha, but not quite ready for a major release, then we could
bump the patch release again,

    $ release patch
    v0.0.2

this time we chose to omit the `-info` flag, and the pre-release information.
Bumping a release version will always drop the pre-release and build information
from the prior release.

Some more changes have been made to the software and it is stable enough for a
major release. So, let's bump the major version, and denote this release as a
release candidate,

    $ release major rc1
    v1.0.0-rc1

as you can see, bumping the major version has reset the minor and patch versions
to zero. Bumping just the minor version will also reset the patch to zero too.

This has hopefully given you a quick rundown of how this tool can be used to
prepare release notes and source code artifacts with ease.
