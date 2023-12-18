# Release process

Project's release cadence is quarterly. The release process is tracked as below:
* Run validation on main branch
* Create release-<version> branch
* Draft release notes, review
* Update main README for supported versions and docs URL
* Final validation
* Publish release

Once the content is available in the main branch and validation PASSes, release branch will be created (e.g. release-v0.3.0). The HEAD of release branch will also be tagged with the corresponding tag (e.g. release-v0.3.0).

During the release creation, the project's documentation, deployment files etc. will be changed to point to the newly created version.

Patch releases (e.g. release-v0.3.1) are done on a need basis if there are security issues or minor fixes for specific supported version. Fixes are always cherry-picked from the main branch to the release branches.
